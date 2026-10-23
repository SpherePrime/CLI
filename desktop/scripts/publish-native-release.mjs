import { readdir, readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'
const tag = process.argv[2]
if (!/^v\d+\.\d+\.\d+$/.test(tag || '')) throw new Error('A stable release tag is required')
const directory = new URL('../release/native/', import.meta.url)
const platforms = {}
for (const platform of ['windows-x86_64', 'linux-x86_64']) {
  Object.assign(platforms, JSON.parse(await readFile(new URL(`${platform}.json`, directory), 'utf8')))
  if (!platforms[platform]?.signature) throw new Error(`Missing signature for ${platform}`)
}
await writeFile(new URL('latest.json', directory), JSON.stringify({ version: tag.slice(1), platforms }, null, 2) + '\n')
const assets = (await readdir(directory)).filter((name) => name === 'latest.json' || /\.(exe|AppImage|deb|rpm|sig)$/.test(name))
execFileSync('gh', ['release', 'upload', tag, ...assets.map((name) => fileURLToPath(new URL(name, directory))), '--repo', 'SpherePrime/CLI', '--clobber'], { stdio: 'inherit' })
