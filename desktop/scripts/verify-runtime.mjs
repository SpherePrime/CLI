import { createHash } from 'node:crypto'
import { readFile } from 'node:fs/promises'
const manifest = JSON.parse(
  await readFile(new URL('../runtime/manifest.json', import.meta.url), 'utf8')
)
const bytes = await readFile(new URL('../runtime/prime.exe', import.meta.url))
const checksum = createHash('sha256').update(bytes).digest('hex')
if (checksum !== manifest.sha256) throw new Error('Prime runtime checksum mismatch')
if (manifest.platform !== process.platform || manifest.arch !== process.arch)
  throw new Error('Prime runtime architecture mismatch')
console.log(`Prime ${manifest.version} (${manifest.commit}) verified: ${checksum}`)
