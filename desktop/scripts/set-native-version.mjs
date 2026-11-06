import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname } from 'node:path'
import { crateName, versionFiles, writeVersion } from './native-version.mjs'

const version = process.argv[2]?.replace(/^v/, '')
if (!/^\d+\.\d+\.\d+$/.test(version || '')) throw new Error('A stable release version is required')

for (const path of versionFiles) {
  if (!existsSync(path)) continue
  const name = path.endsWith('Cargo.lock')
    ? crateName(readFileSync(`${dirname(path)}/Cargo.toml`, 'utf8'))
    : undefined
  writeFileSync(path, writeVersion(path, readFileSync(path, 'utf8'), version, name))
}
