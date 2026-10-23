import { readFileSync, writeFileSync } from 'node:fs'
const version = process.argv[2]?.replace(/^v/, '')
if (!/^\d+\.\d+\.\d+$/.test(version || '')) throw new Error('A stable release version is required')
for (const path of ['package.json', 'package-lock.json', 'src-tauri/tauri.conf.json']) {
  const data = JSON.parse(readFileSync(path, 'utf8'))
  data.version = version
  if (data.packages?.['']) data.packages[''].version = version
  writeFileSync(path, JSON.stringify(data, null, 2) + '\n')
}
const cargo = 'src-tauri/Cargo.toml'
writeFileSync(cargo, readFileSync(cargo, 'utf8').replace(/^(version\s*=\s*)"[^"]+"/m, `$1"${version}"`))
