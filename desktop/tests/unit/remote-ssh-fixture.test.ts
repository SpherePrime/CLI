import { readFile } from 'node:fs/promises'
import { createConnection } from 'node:net'
import { resolve } from 'node:path'
import { Client } from 'ssh2'
import { expect, it } from 'vitest'
import { createSshLinuxFixture, detectSshLinuxFixture } from '../fixtures/ssh-linux'

const support = await detectSshLinuxFixture()
it.skipIf(!support.available)(
  'starts an isolated SSH server, transfers the Linux runtime and stops its listener',
  async () => {
    const fixture = await createSshLinuxFixture()
    const client = new Client()
    try {
      const privateKey = await readFile(fixture.keyPath)
      await new Promise<void>((ready, reject) => {
        client.once('ready', ready).once('error', reject)
        client.connect({
          host: fixture.host,
          port: fixture.port,
          username: fixture.user,
          privateKey,
          readyTimeout: 10000
        })
      })
      const sftp = await new Promise<import('ssh2').SFTPWrapper>((ready, reject) =>
        client.sftp((error, result) => (error ? reject(error) : ready(result)))
      )
      const target = `${fixture.root}/prime`
      await new Promise<void>((ready, reject) =>
        sftp.fastPut(
          resolve('runtime/remotes/linux-x64/prime'),
          target,
          { mode: 0o700 },
          (error) => (error ? reject(error) : ready())
        )
      )
      sftp.end()
      const version = await new Promise<string>((ready, reject) =>
        client.exec(`${target} --version`, (error, stream) => {
          if (error) return reject(error)
          let output = ''
          stream.on('data', (chunk: Buffer) => {
            output += chunk.toString()
          })
          stream.on('close', (code: number) =>
            code === 0 ? ready(output) : reject(new Error(output))
          )
        })
      )
      expect(version.trim().length).toBeGreaterThan(0)
      const healthScript = `import os,socket,subprocess,time,urllib.request
base=${JSON.stringify(fixture.linuxDir)}
env=dict(os.environ,PRIME_GLOBAL_CONFIG=base+'/config',PRIME_GLOBAL_DATA=base+'/data',PRIME_CACHE_DIR=base+'/cache',PRIME_DISABLE_PROVIDER_AUTO_UPDATE='1')
with socket.socket() as listener:
 listener.bind(('127.0.0.1',0))
 port=listener.getsockname()[1]
with open(base+'/server.log','w') as log:
 process=subprocess.Popen([${JSON.stringify(target)},'server','--host',f'tcp://127.0.0.1:{port}'],env=env,stdout=log,stderr=log)
 try:
  for _ in range(100):
   try:
    with urllib.request.urlopen(f'http://127.0.0.1:{port}/v1/health',timeout=.5) as response:
     assert response.status == 200
     print('healthy')
     break
   except OSError:
    if process.poll() is not None: raise RuntimeError(open(base+'/server.log').read())
    time.sleep(.1)
  else: raise RuntimeError('Prime health check timed out: '+open(base+'/server.log').read())
 finally:
  process.terminate()
  process.wait(timeout=8)
`
      const command = `python3 -c '${healthScript.replaceAll("'", "'\\''")}'`
      const health = await new Promise<string>((ready, reject) =>
        client.exec(command, (error, stream) => {
          if (error) return reject(error)
          let output = ''
          stream.on('data', (chunk: Buffer) => {
            output += chunk.toString()
          })
          stream.stderr.on('data', (chunk: Buffer) => {
            output += chunk.toString()
          })
          stream.on('close', (code: number) =>
            code === 0 ? ready(output) : reject(new Error(output))
          )
        })
      )
      expect(health.trim()).toBe('healthy')
      expect(await fixture.execute('pwd')).toContain('/tmp/prime-ssh-fixture-')
      client.end()
    } finally {
      client.destroy()
      await fixture.close()
    }
    await expect(readFile(fixture.keyPath)).rejects.toMatchObject({ code: 'ENOENT' })
    await fixture.close()
  },
  60000
)
