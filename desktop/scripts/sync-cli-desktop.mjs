import { execFileSync } from 'node:child_process'
import { copyFile, mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { dirname, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { existsSync } from 'node:fs'

const source = fileURLToPath(new URL('..', import.meta.url))
const parent = resolve(source, '..')
const cli = resolve(
  process.env.PRIME_CLI_SOURCE ||
    (existsSync(resolve(parent, 'go.mod')) ? parent : resolve(parent, 'CLI'))
)
const destination = resolve(cli, 'desktop')
if (resolve(source) === destination) {
  process.stdout.write('Desktop sources already belong to the CLI repository\n')
  process.exit(0)
}
const target = (value) => {
  const path = resolve(destination, value)
  if (!path.startsWith(destination + sep)) throw new Error('Invalid Desktop source path')
  return path
}
const files = execFileSync(
  'git',
  ['ls-files', '-z', '--cached', '--others', '--exclude-standard'],
  { cwd: source, encoding: 'utf8' }
)
  .split('\0')
  .filter(Boolean)
  .filter(
    (file) =>
      !file.startsWith('release/') &&
      file !== 'runtime/manifest.json' &&
      !file.startsWith('installer/payload/') &&
      !file.startsWith('installer/out/') &&
      !file.startsWith('out/') &&
      !file.startsWith('.superpowers/') &&
      file !== 'CHATS.md'
  )
const manifest = target('.source-files.json')
const previous = await readFile(manifest, 'utf8')
  .then(JSON.parse)
  .catch((error) => {
    if (error.code === 'ENOENT') return []
    throw error
  })
for (const file of previous) if (!files.includes(file)) await rm(target(file), { force: true })
for (const file of files) {
  await mkdir(dirname(target(file)), { recursive: true })
  await copyFile(resolve(source, file), target(file))
}
await writeFile(manifest, JSON.stringify(files.sort(), null, 2) + '\n')
process.stdout.write(`Synced ${files.length} Desktop source files to ${destination}\n`)
