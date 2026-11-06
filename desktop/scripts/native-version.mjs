const jsonFiles = ['package.json', 'package-lock.json', 'src-tauri/tauri.conf.json', 'installer/src-tauri/tauri.conf.json']
const crates = ['src-tauri', 'installer/src-tauri']

export const versionFiles = [
  ...jsonFiles,
  ...crates.flatMap(crate => [`${crate}/Cargo.toml`, `${crate}/Cargo.lock`]),
]

const escapeRegExp = text => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

const lockPattern = name => new RegExp(`(name = "${escapeRegExp(name)}"\\r?\\nversion = )"([^"]+)"`)

export const crateName = manifest => /^name\s*=\s*"([^"]+)"/m.exec(manifest)?.[1]

export function readVersion(path, content, name) {
  if (path.endsWith('.json')) return JSON.parse(content).version
  if (path.endsWith('Cargo.toml')) return /^version\s*=\s*"([^"]+)"/m.exec(content)?.[1]
  if (path.endsWith('Cargo.lock') && name) return lockPattern(name).exec(content)?.[2]
  return undefined
}

export function writeVersion(path, content, version, name) {
  if (path.endsWith('.json')) {
    const data = JSON.parse(content)
    data.version = version
    if (data.packages?.['']) data.packages[''].version = version
    return JSON.stringify(data, null, 2) + '\n'
  }
  if (path.endsWith('Cargo.toml')) return content.replace(/^(version\s*=\s*)"[^"]+"/m, `$1"${version}"`)
  if (path.endsWith('Cargo.lock') && name) return content.replace(lockPattern(name), `$1"${version}"`)
  return content
}
