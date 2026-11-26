import { createHash } from 'node:crypto'
import { chmodSync, copyFileSync, existsSync, mkdirSync, readFileSync, renameSync, rmSync, statSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const desktopDir = path.resolve(scriptDir, '..')
const repoRoot = path.resolve(desktopDir, '..')
const outDir = path.join(desktopDir, 'src-tauri', 'asr')
const siblingRepo = path.resolve(repoRoot, '..', 'prime-asr')

const RELEASE_REPO = 'SpherePrime/prime-asr'
const RELEASE_TAG = 'v0.1.0'
const MODEL_COMMIT = 'e034289132667fa8e87dd8d474a4dc59d447b2dc'

const MODELS = [
  {
    name: 'en-ru-uk.tzf.gz',
    sha256: '5215d4e6c90b9c119f1267862d83833ba5b6ea9c28c5df4ebefe4265e26f2f53'
  },
  {
    name: 'silero_vad.onnx.gz',
    sha256: 'cf8770c9376f65ffb8c6667bbca0edb993d9768902b3a0200b1041bcb16adaa4'
  }
]

function targetInfo() {
  const platform = process.env.PRIME_ASR_PLATFORM || process.platform
  const arch = process.env.PRIME_ASR_ARCH || process.arch
  const isWin = platform === 'win32' || platform === 'windows'
  const goArch = arch === 'arm64' || arch === 'aarch64' ? 'arm64' : 'amd64'
  const binName = isWin ? 'prime-asr.exe' : 'prime-asr'
  const assetName = isWin
    ? `prime-asr-windows-${goArch}.exe`
    : `prime-asr-linux-${goArch}`
  return { isWin, binName, assetName }
}

function sha256File(filePath) {
  const hash = createHash('sha256')
  hash.update(readFileSync(filePath))
  return hash.digest('hex')
}

function isValidFile(filePath, expectedSha) {
  if (!existsSync(filePath) || statSync(filePath).size === 0) return false
  if (!expectedSha) return true
  return sha256File(filePath).toLowerCase() === expectedSha.toLowerCase()
}

async function downloadFile(urls, dest, expectedSha) {
  let lastError
  for (const url of urls) {
    try {
      const response = await fetch(url, { redirect: 'follow' })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status} for ${url}`)
      }
      const data = Buffer.from(await response.arrayBuffer())
      if (expectedSha) {
        const actual = createHash('sha256').update(data).digest('hex')
        if (actual.toLowerCase() !== expectedSha.toLowerCase()) {
          throw new Error(`SHA-256 mismatch for ${url}: got ${actual}`)
        }
      }
      const tmp = `${dest}.part`
      writeFileSync(tmp, data)
      if (existsSync(dest)) rmSync(dest, { force: true })
      renameSync(tmp, dest)
      return
    } catch (error) {
      lastError = error
    }
  }
  throw lastError || new Error(`Failed to download ${dest}`)
}

async function prepare() {
  mkdirSync(outDir, { recursive: true })
  const { isWin, binName, assetName } = targetInfo()
  const destBin = path.join(outDir, binName)
  const localBin = path.join(siblingRepo, 'target', 'release', binName)

  if (existsSync(localBin) && statSync(localBin).size > 0) {
    copyFileSync(localBin, destBin)
    if (!isWin) chmodSync(destBin, 0o755)
  } else if (!isValidFile(destBin, null)) {
    const releaseUrl = `https://github.com/${RELEASE_REPO}/releases/download/${RELEASE_TAG}/${assetName}`
    await downloadFile([releaseUrl], destBin, null)
    if (!isWin) chmodSync(destBin, 0o755)
  }

  for (const model of MODELS) {
    const destModel = path.join(outDir, model.name)
    if (isValidFile(destModel, model.sha256)) continue
    const localModel = path.join(siblingRepo, 'models', model.name)
    if (isValidFile(localModel, model.sha256)) {
      copyFileSync(localModel, destModel)
      continue
    }
    const urls = [
      `https://github.com/${RELEASE_REPO}/releases/download/${RELEASE_TAG}/${model.name}`,
      `https://raw.githubusercontent.com/dkaraush/tinyasr/${MODEL_COMMIT}/models/${model.name}`
    ]
    await downloadFile(urls, destModel, model.sha256)
  }

  console.log(`Prepared ASR bundle in ${outDir}`)
}

prepare().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error))
  process.exit(1)
})
