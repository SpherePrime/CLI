import { mkdtemp, readdir, readFile, rm, writeFile } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execFileSync, spawnSync } from 'node:child_process'
const tag = process.argv[2]
if (!/^v\d+\.\d+\.\d+$/.test(tag || '')) throw new Error('A stable release tag is required')
const repository = 'SpherePrime/CLI'
const version = tag.slice(1)
const directory = fileURLToPath(new URL('../release/native/', import.meta.url))
const staged = {}
for (const platform of ['windows-x86_64', 'linux-x86_64']) {
  const path = join(directory, `${platform}.json`)
  if (!existsSync(path)) continue
  Object.assign(staged, JSON.parse(await readFile(path, 'utf8')))
  if (!staged[platform]?.signature) throw new Error(`Missing signature for ${platform}`)
}
if (Object.keys(staged).length === 0) throw new Error('No signed desktop platform is staged')
const download = await mkdtemp(join(tmpdir(), 'prime-latest-'))
let published = {}
try {
  const result = spawnSync('gh', ['release', 'download', tag, '--repo', repository, '--pattern', 'latest.json', '--dir', download], { stdio: 'ignore' })
  if (result.status === 0) {
    const current = JSON.parse(await readFile(join(download, 'latest.json'), 'utf8'))
    if (current.version === version) published = current.platforms || {}
  }
} finally {
  await rm(download, { recursive: true, force: true })
}
const platforms = { ...published, ...staged }
await writeFile(join(directory, 'latest.json'), JSON.stringify({ version, platforms }, null, 2) + '\n')
const assets = (await readdir(directory)).filter((name) => name === 'latest.json' || /\.(exe|AppImage|deb|rpm|sig)$/.test(name))
execFileSync('gh', ['release', 'upload', tag, ...assets.map((name) => join(directory, name)), '--repo', repository, '--clobber'], { stdio: 'inherit' })
console.log(`latest.json ${version}: ${Object.keys(platforms).sort().join(', ')}`)
