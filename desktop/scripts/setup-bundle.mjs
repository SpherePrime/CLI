import { basename } from 'node:path'

export function setupPayloads(platform, version, files) {
  const patterns = platform === 'win32'
    ? { 'app.exe': `_${version}_x64-setup.exe` }
    : platform === 'linux' ? {
      'app.AppImage': `_${version}_amd64.AppImage`,
      'app.deb': `_${version}_amd64.deb`,
      'app.rpm': `-${version}-1.x86_64.rpm`
    } : null
  if (!patterns) throw new Error(`Unsupported setup platform: ${platform}`)
  return Object.fromEntries(Object.entries(patterns).map(([name, suffix]) => {
    const matches = files.filter((file) => file.endsWith(suffix) && !basename(file).includes('Setup'))
    if (matches.length !== 1) throw new Error(`Expected one ${name} payload for ${version}, found ${matches.length}`)
    return [name, matches[0]]
  }))
}
