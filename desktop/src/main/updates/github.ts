export type DesktopRelease = { version: string; url: string }
type ReleaseAssets = { metadata: string; installer: RegExp; binary: (version: string) => string }
const stableVersion = (value: string) => /^v?(\d+)\.(\d+)\.(\d+)$/.exec(value)
export function releaseAssets(platform: NodeJS.Platform = process.platform): ReleaseAssets {
  if (platform === 'linux')
    return {
      metadata: 'latest-linux.yml',
      installer: /^PrimeDesktop-.*\.AppImage$/i,
      binary: (version) => `PrimeDesktop-${version}.AppImage`
    }
  return {
    metadata: 'latest.yml',
    installer: /^(Prime[ -]Desktop[ -]Setup|PrimeDesktop-Update-).*\.exe$/i,
    binary: (version) => `PrimeDesktop-Update-${version}.exe`
  }
}
export function selectDesktopRelease(
  releases: any[],
  current: string,
  assets = releaseAssets()
): DesktopRelease | null {
  const baseline = stableVersion(current)
  if (!baseline) throw new Error('Invalid installed version')
  const compare = (a: string, b: string) => {
    const left = stableVersion(a)!,
      right = stableVersion(b)!
    for (let index = 1; index <= 3; index++) {
      const difference = Number(left[index]) - Number(right[index])
      if (difference) return difference
    }
    return 0
  }
  const candidates = releases
    .filter(
      (release) =>
        !release.draft &&
        !release.prerelease &&
        typeof release.tag_name === 'string' &&
        stableVersion(release.tag_name) &&
        Array.isArray(release.assets) &&
        release.assets.some((asset: any) => asset.name === assets.metadata) &&
        release.assets.some((asset: any) => assets.installer.test(asset.name)) &&
        compare(release.tag_name, current) > 0
    )
    .sort((a, b) => compare(b.tag_name, a.tag_name))
  const selected = candidates[0]
  return selected
    ? {
        version: selected.tag_name.replace(/^v/, ''),
        url: `https://github.com/SpherePrime/CLI/releases/download/${encodeURIComponent(selected.tag_name)}/`
      }
    : null
}
export async function discoverDesktopRelease(
  current: string,
  assets = releaseAssets()
): Promise<DesktopRelease | null> {
  const request = (url: string, method = 'GET') =>
    fetch(url, {
      method,
      headers: { 'User-Agent': 'SpherePrime-Desktop' },
      signal: AbortSignal.timeout(20000)
    })
  const response = await request('https://github.com/SpherePrime/CLI/releases.atom')
  if (!response.ok) throw new Error(`GitHub: HTTP ${response.status}`)
  const feed = await response.text()
  if (feed.length > 2 * 1024 * 1024) throw new Error('Invalid release feed')
  let releases = [
    ...feed.matchAll(
      /href="https:\/\/github\.com\/SpherePrime\/CLI\/releases\/tag\/(v?\d+\.\d+\.\d+)"/g
    )
  ].map((match) => ({
    tag_name: match[1],
    assets: [{ name: assets.metadata }, { name: assets.binary(match[1].replace(/^v/, '')) }]
  }))
  for (;;) {
    const release = selectDesktopRelease(releases, current, assets)
    if (!release) return null
    const metadata = await request(`${release.url}${assets.metadata}`)
    if (metadata.status !== 404) {
      if (!metadata.ok) throw new Error(`GitHub: HTTP ${metadata.status}`)
      const content = await metadata.text()
      if (
        content.length > 65536 ||
        /^version:\s*['"]?([\d.]+)/m.exec(content)?.[1] !== release.version
      )
        throw new Error('Invalid Desktop release metadata')
      const binary = await request(`${release.url}${assets.binary(release.version)}`, 'HEAD')
      if (binary.ok) return release
      if (binary.status !== 404) throw new Error(`GitHub: HTTP ${binary.status}`)
    }
    releases = releases.filter((item) => item.tag_name.replace(/^v/, '') !== release.version)
  }
}
