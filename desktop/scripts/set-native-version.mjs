import { existsSync, readFileSync, writeFileSync } from 'node:fs'
const version = process.argv[2]?.replace(/^v/, '')
if (!/^\d+\.\d+\.\d+$/.test(version || '')) throw new Error('A stable release version is required')
for (const path of ['package.json', 'package-lock.json', 'src-tauri/tauri.conf.json', 'installer/src-tauri/tauri.conf.json']) {
  const data = JSON.parse(readFileSync(path, 'utf8'))
  data.version = version
  if (data.packages?.['']) data.packages[''].version = version
  writeFileSync(path, JSON.stringify(data, null, 2) + '\n')
}
for (const crate of ['src-tauri', 'installer/src-tauri']) {
  const cargo = `${crate}/Cargo.toml`
  const manifest = readFileSync(cargo, 'utf8')
  writeFileSync(cargo, manifest.replace(/^(version\s*=\s*)"[^"]+"/m, `$1"${version}"`))
  const name = /^name\s*=\s*"([^"]+)"/m.exec(manifest)?.[1]
  const lock = `${crate}/Cargo.lock`
  if (!name || !existsSync(lock)) continue
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  writeFileSync(lock, readFileSync(lock, 'utf8').replace(new RegExp(`(name = "${escaped}"\\r?\\nversion = )"[^"]+"`), `$1"${version}"`))
}
