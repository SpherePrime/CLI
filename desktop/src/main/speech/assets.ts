import { createHash } from 'node:crypto'
import { createReadStream, createWriteStream } from 'node:fs'
import { mkdir, rename, rm, stat, writeFile, readFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { Readable } from 'node:stream'
import { pipeline } from 'node:stream/promises'

const repository = 'https://huggingface.co/supertone-oss-archive/supertonic-3/resolve/aafc6e32416a594460b32413efc49d7fe4ce6d46'

type Asset = { path: string; size: number; sha256?: string; blob?: string }

export const speechAssets: Asset[] = [
  { path: 'onnx/tts.json', size: 8253, blob: '28575b0793c92607a0f0a292550df9bd17709a79' },
  { path: 'onnx/unicode_indexer.json', size: 277676, blob: '3aaf4d33024aff0be455633c24b3636b3a810150' },
  { path: 'onnx/duration_predictor.onnx', size: 3700147, sha256: 'c3eb91414d5ff8a7a239b7fe9e34e7e2bf8a8140d8375ffb14718b1c639325db' },
  { path: 'onnx/text_encoder.onnx', size: 36416150, sha256: 'c7befd5ea8c3119769e8a6c1486c4edc6a3bc8365c67621c881bbb774b9902ff' },
  { path: 'onnx/vector_estimator.onnx', size: 256534781, sha256: '883ac868ea0275ef0e991524dc64f16b3c0376efd7c320af6b53f5b780d7c61c' },
  { path: 'onnx/vocoder.onnx', size: 101424195, sha256: '085de76dd8e8d5836d6ca66826601f615939218f90e519f70ee8a36ed2a4c4ba' },
  { path: 'voice_styles/F1.json', size: 292046, blob: '421365f307ed1535d8da16845031d0e9fa3e60c5' },
  { path: 'voice_styles/F2.json', size: 292423, blob: 'b09a3a433df8a4856d1d7d64736a56ddf37a4ea3' },
  { path: 'voice_styles/M1.json', size: 291748, blob: 'bddfff85a7c4c140c11620bd7005d86681dccdac' },
  { path: 'voice_styles/M2.json', size: 292055, blob: '602ff6c4d6d48cdb71c37461f7c175d71fe8b34b' }
]

const totalSize = speechAssets.reduce((sum, asset) => sum + asset.size, 0)
const marker = 'verified.json'

export function speechModelDirectory() {
  const base = process.env.LOCALAPPDATA || join(process.env.USERPROFILE || '.', 'AppData', 'Local')
  return join(base, 'Prime Desktop', 'speech', 'supertonic-3')
}

async function digest(path: string, asset: Asset) {
  const hash = asset.sha256 ? createHash('sha256') : createHash('sha1').update(`blob ${asset.size}\0`)
  for await (const chunk of createReadStream(path)) hash.update(chunk)
  return hash.digest('hex')
}

async function valid(path: string, asset: Asset) {
  try {
    if ((await stat(path)).size !== asset.size) return false
    return (await digest(path, asset)) === (asset.sha256 ?? asset.blob)
  } catch {
    return false
  }
}

export async function speechModelReady(directory = speechModelDirectory()) {
  try {
    const saved = JSON.parse(await readFile(join(directory, marker), 'utf8'))
    if (saved.total !== totalSize) return false
    for (const asset of speechAssets) if ((await stat(join(directory, asset.path))).size !== asset.size) return false
    return true
  } catch {
    return false
  }
}

export async function downloadSpeechModel(
  progress: (percent: number) => void,
  options: { directory?: string; signal?: AbortSignal } = {}
) {
  const directory = options.directory ?? speechModelDirectory()
  if (await speechModelReady(directory)) {
    progress(100)
    return directory
  }
  let done = 0
  for (const asset of speechAssets) {
    const target = join(directory, asset.path)
    if (await valid(target, asset)) {
      done += asset.size
      progress((done / totalSize) * 100)
      continue
    }
    await mkdir(dirname(target), { recursive: true })
    const temporary = `${target}.download`
    let attempt = 0
    while (true) {
      let received = 0
      try {
        const response = await fetch(`${repository}/${asset.path}`, { signal: options.signal, redirect: 'follow' })
        if (!response.ok || !response.body) throw new Error(`HTTP ${response.status}`)
        const body = Readable.fromWeb(response.body as any)
        body.on('data', (chunk: Buffer) => {
          received += chunk.length
          progress(((done + received) / totalSize) * 100)
        })
        await pipeline(body, createWriteStream(temporary))
        if (!(await valid(temporary, asset))) throw new Error('checksum')
        await rename(temporary, target)
        break
      } catch (error) {
        await rm(temporary, { force: true })
        if (options.signal?.aborted || ++attempt >= 3)
          throw new Error(`Не удалось скачать голосовую модель (${asset.path}): ${(error as Error).message}`)
      }
    }
    done += asset.size
    progress((done / totalSize) * 100)
  }
  await writeFile(join(directory, marker), JSON.stringify({ total: totalSize, at: Date.now() }))
  return directory
}
