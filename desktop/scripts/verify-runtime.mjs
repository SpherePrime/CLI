import { createHash } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { runtimeTargets } from './runtime-targets.mjs'

const runtimeRoot = process.argv[2]
  ? resolve(process.argv[2])
  : fileURLToPath(new URL('../runtime/', import.meta.url))
let desktopManifest
for (const target of runtimeTargets) {
  const directory = join(runtimeRoot, target.directory)
  const manifest = JSON.parse(await readFile(join(directory, 'manifest.json'), 'utf8'))
  const bytes = await readFile(join(directory, target.binary))
  const checksum = createHash('sha256').update(bytes).digest('hex')
  const identity = `${target.platform}-${target.arch}`
  if (checksum !== manifest.sha256) throw new Error(`Prime runtime ${identity} checksum mismatch`)
  if (
    manifest.platform !== target.platform ||
    manifest.arch !== target.arch ||
    manifest.protocolVersion !== 1
  )
    throw new Error(`Prime runtime ${identity} metadata mismatch`)
  if (!desktopManifest) {
    if (manifest.platform !== process.platform || manifest.arch !== process.arch)
      throw new Error('Prime runtime architecture mismatch')
    if (
      typeof manifest.version !== 'string' ||
      !manifest.version ||
      typeof manifest.commit !== 'string' ||
      !manifest.commit
    )
      throw new Error('Prime runtime revision missing')
    desktopManifest = manifest
  } else if (
    manifest.version !== desktopManifest.version ||
    manifest.commit !== desktopManifest.commit
  ) {
    throw new Error(`Prime runtime ${identity} revision mismatch`)
  }
  console.log(`Prime ${manifest.version} (${manifest.commit}) ${identity} verified: ${checksum}`)
}
