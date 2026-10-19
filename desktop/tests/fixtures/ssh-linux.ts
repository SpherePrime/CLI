import { execFile } from 'node:child_process'
import { mkdtemp, rm, writeFile } from 'node:fs/promises'
import { createConnection } from 'node:net'
import { tmpdir } from 'node:os'
import { basename, dirname, join, resolve } from 'node:path'
import { setTimeout as delay } from 'node:timers/promises'
import { promisify } from 'node:util'

const executeFile = promisify(execFile)
const quote = (value: string) => `'${value.replaceAll("'", "'\\''")}'`

async function wsl(command: string) {
  const { stdout } = await executeFile('wsl.exe', ['-d', 'Ubuntu', '--', 'sh', '-c', command], {
    timeout: 20000,
    maxBuffer: 4 * 1024 * 1024,
    encoding: 'utf8'
  })
  return stdout.replaceAll('\r', '').trimEnd()
}

export async function detectSshLinuxFixture() {
  if (process.platform !== 'win32')
    return {
      available: false,
      reason: 'The isolated SSH fixture requires Windows with WSL Ubuntu.'
    }
  try {
    await wsl(
      'test "$(id -u)" = 0 && test -x /usr/sbin/sshd && command -v ssh-keygen && command -v python3 && test -d /run/sshd'
    )
    return { available: true }
  } catch {
    return {
      available: false,
      reason: 'WSL Ubuntu with root, sshd, ssh-keygen, Python 3 and /run/sshd is unavailable.'
    }
  }
}

async function reachable(host: string, port: number) {
  return new Promise<boolean>((resolve) => {
    const socket = createConnection({ host, port })
    const finish = (result: boolean) => {
      socket.destroy()
      resolve(result)
    }
    socket.setTimeout(500)
    socket.once('connect', () => finish(true))
    socket.once('timeout', () => finish(false))
    socket.once('error', () => finish(false))
  })
}

export interface SshLinuxFixture {
  host: string
  port: number
  user: 'root'
  root: string
  keyPath: string
  linuxDir: string
  close(): Promise<void>
  execute(command: string): Promise<string>
}

export async function createSshLinuxFixture(): Promise<SshLinuxFixture> {
  const support = await detectSshLinuxFixture()
  if (!support.available) throw new Error(support.reason)
  const linuxDir = await wsl('mktemp -d /tmp/prime-ssh-fixture-XXXXXXXXXX')
  if (!/^\/tmp\/prime-ssh-fixture-[A-Za-z0-9]+$/.test(linuxDir))
    throw new Error('Invalid SSH fixture directory')
  const windowsDir = await mkdtemp(join(tmpdir(), 'prime-ssh-fixture-'))
  const keyPath = join(windowsDir, 'client_ed25519')
  const root = `${linuxDir}/project`
  const configPath = `${linuxDir}/sshd_config`
  const pidPath = `${linuxDir}/sshd.pid`
  let closed = false
  const close = async () => {
    if (closed) return
    const cleanup = `import os,pathlib,signal,time
base=pathlib.Path(${JSON.stringify(linuxDir)})
assert base.parent == pathlib.Path('/tmp') and base.name.startswith('prime-ssh-fixture-')
pidfile=base/'sshd.pid'
if pidfile.exists():
 pid=int(pidfile.read_text())
 proc=pathlib.Path('/proc')/str(pid)
 if proc.exists():
  assert os.path.realpath(proc/'exe') == '/usr/sbin/sshd'
  assert str(base/'sshd_config').encode() in (proc/'cmdline').read_bytes()
  children={pid}
  for _ in range(12):
   for entry in pathlib.Path('/proc').iterdir():
    if entry.name.isdigit():
     try:
      status=(entry/'stat').read_text().rsplit(')',1)[1].split()
      if int(status[1]) in children: children.add(int(entry.name))
     except (FileNotFoundError,ProcessLookupError): pass
  for child in sorted(children,reverse=True):
   try: os.kill(child,signal.SIGTERM)
   except ProcessLookupError: pass
  for _ in range(100):
   try:
    state=(proc/'stat').read_text().rsplit(')',1)[1].split()[0]
    if state == 'Z': break
   except FileNotFoundError: break
   time.sleep(.02)
  else: raise RuntimeError('Fixture sshd did not stop')
import shutil
if base.exists(): shutil.rmtree(base)
`
    await wsl(`python3 -c ${quote(cleanup)}`)
    if (
      dirname(resolve(windowsDir)) !== resolve(tmpdir()) ||
      !basename(windowsDir).startsWith('prime-ssh-fixture-')
    )
      throw new Error('Invalid Windows SSH fixture directory')
    await rm(windowsDir, { recursive: true, force: true })
    closed = true
  }
  try {
    await wsl(
      `mkdir -p ${quote(root)} ${quote(linuxDir + '/config')} ${quote(linuxDir + '/data')} ${quote(linuxDir + '/cache')}; ssh-keygen -q -t ed25519 -N '' -f ${quote(linuxDir + '/host_ed25519')}; ssh-keygen -q -t ed25519 -N '' -f ${quote(linuxDir + '/client_ed25519')}; cp ${quote(linuxDir + '/client_ed25519.pub')} ${quote(linuxDir + '/authorized_keys')}`
    )
    await writeFile(keyPath, (await wsl(`cat ${quote(linuxDir + '/client_ed25519')}`)) + '\n', {
      mode: 0o600
    })
    const port = Number(
      await wsl(
        `python3 -c ${quote('import socket; s=socket.socket(); s.bind(("0.0.0.0",0)); print(s.getsockname()[1]); s.close()')}`
      )
    )
    if (!Number.isInteger(port) || port < 1024 || port > 65535)
      throw new Error('Invalid SSH fixture port')
    const configuration = [
      `Port ${port}`,
      'ListenAddress 0.0.0.0',
      `HostKey ${linuxDir}/host_ed25519`,
      `PidFile ${pidPath}`,
      `AuthorizedKeysFile ${linuxDir}/authorized_keys`,
      'PermitRootLogin prohibit-password',
      'PubkeyAuthentication yes',
      'PasswordAuthentication no',
      'KbdInteractiveAuthentication no',
      'StrictModes no',
      'UsePAM no',
      'AllowUsers root',
      'AllowTcpForwarding yes',
      'GatewayPorts no',
      'X11Forwarding no',
      'PrintMotd no',
      'Subsystem sftp internal-sftp',
      `SetEnv PRIME_GLOBAL_CONFIG=${linuxDir}/config PRIME_GLOBAL_DATA=${linuxDir}/data PRIME_CACHE_DIR=${linuxDir}/cache PRIME_DISABLE_PROVIDER_AUTO_UPDATE=1`,
      'LogLevel VERBOSE',
      ''
    ].join('\n')
    await wsl(
      `printf %s ${quote(configuration)} > ${quote(configPath)}; /usr/sbin/sshd -t -f ${quote(configPath)}; /usr/sbin/sshd -f ${quote(configPath)} -E ${quote(linuxDir + '/sshd.log')}`
    )
    const candidates = ['127.0.0.1', ...(await wsl('hostname -I')).split(/\s+/).filter(Boolean)]
    let host = ''
    for (let attempt = 0; attempt < 30 && !host; attempt++) {
      for (const candidate of candidates) {
        if (await reachable(candidate, port)) {
          host = candidate
          break
        }
      }
      if (!host) await delay(100)
    }
    if (!host)
      throw new Error(
        `SSH fixture is not reachable: ${await wsl(`cat ${quote(linuxDir + '/sshd.log')}`)}`
      )
    return {
      host,
      port,
      user: 'root',
      root,
      keyPath,
      linuxDir,
      close,
      execute: (command) => wsl(`cd ${quote(root)} && ${command}`)
    }
  } catch (error) {
    await close()
    throw error
  }
}
