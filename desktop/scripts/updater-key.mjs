import { spawnSync } from 'node:child_process'
import { createPublicKey, verify } from 'node:crypto'
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { homedir, tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const DEFAULT_UPDATER_ENDPOINT = 'https://github.com/SpherePrime/CLI/releases/latest/download/latest.json'
const ED25519_SPKI_PREFIX = Buffer.from('302a300506032b6570032100', 'hex')

function decodeStrictBase64(value, label) {
  if (typeof value !== 'string' || !value.trim()) {
    throw new Error(`Invalid ${label}: empty value`)
  }
  const compact = value.replace(/\s+/g, '')
  if (!/^[A-Za-z0-9+/]+={0,2}$/.test(compact)) {
    throw new Error(`Invalid ${label}: malformed base64`)
  }
  return Buffer.from(compact, 'base64')
}

function readTrimmedFile(path) {
  return existsSync(path) ? readFileSync(path, 'utf8').trim() : undefined
}

function formatLittleEndianKeyId(bytes) {
  return Buffer.from(bytes).reverse().toString('hex').toUpperCase()
}

export function parseMinisignPublicKey(encoded) {
  const decoded = decodeStrictBase64(encoded, 'minisign public key').toString('utf8')
  const lines = decoded.trim().split(/\r?\n/).filter(Boolean)
  if (lines.length < 2 || !lines[0].startsWith('untrusted comment:')) {
    throw new Error('Invalid minisign public key: missing untrusted comment or key payload')
  }
  const raw = decodeStrictBase64(lines[1], 'minisign public key payload')
  if (raw.length !== 42) {
    throw new Error(`Invalid minisign public key payload length: expected 42 bytes, got ${raw.length}`)
  }
  const algorithm = raw.subarray(0, 2).toString('ascii')
  if (algorithm !== 'Ed') {
    throw new Error(`Unsupported minisign public key algorithm: ${algorithm}`)
  }
  const keyId = formatLittleEndianKeyId(raw.subarray(2, 10))
  const publicKey = raw.subarray(10, 42)
  return {
    comment: lines[0],
    algorithm,
    keyId,
    publicKey
  }
}

export function parseMinisignSignature(encoded) {
  const decoded = decodeStrictBase64(encoded, 'minisign signature').toString('utf8')
  const lines = decoded.trim().split(/\r?\n/).filter(Boolean)
  if (lines.length < 2 || !lines[0].startsWith('untrusted comment:')) {
    throw new Error('Invalid minisign signature: missing untrusted comment or signature payload')
  }
  const raw = decodeStrictBase64(lines[1], 'minisign signature payload')
  if (raw.length !== 74) {
    throw new Error(`Invalid minisign signature payload length: expected 74 bytes, got ${raw.length}`)
  }
  const algorithm = raw.subarray(0, 2).toString('ascii')
  if (algorithm !== 'Ed' && algorithm !== 'ED') {
    throw new Error(`Unsupported minisign signature algorithm: ${algorithm}`)
  }
  const keyId = formatLittleEndianKeyId(raw.subarray(2, 10))
  const signature = raw.subarray(10, 74)
  return {
    comment: lines[0],
    algorithm,
    keyId,
    signature
  }
}

export function verifyPublicKeyMatchesSignature(publicKeyEncoded, signatureEncoded, payloadBytes) {
  const parsedPublicKey = parseMinisignPublicKey(publicKeyEncoded)
  const parsedSignature = parseMinisignSignature(signatureEncoded)
  if (parsedPublicKey.keyId !== parsedSignature.keyId) {
    throw new Error(
      `Updater key mismatch: public key ${parsedPublicKey.keyId} does not match signature key ${parsedSignature.keyId}`
    )
  }
  if (parsedSignature.algorithm === 'Ed') {
    const keyObject = createPublicKey({
      key: Buffer.concat([ED25519_SPKI_PREFIX, parsedPublicKey.publicKey]),
      format: 'der',
      type: 'spki'
    })
    const payload = Buffer.isBuffer(payloadBytes) ? payloadBytes : Buffer.from(payloadBytes)
    if (!verify(null, payload, keyObject, parsedSignature.signature)) {
      throw new Error(`Updater signature verification failed for public key ${parsedPublicKey.keyId}`)
    }
  }
  return {
    keyId: parsedPublicKey.keyId,
    algorithm: parsedSignature.algorithm
  }
}

export function resolveUpdaterKeyConfig(root, env = process.env) {
  const keyFile = env.PRIME_UPDATER_KEY_FILE?.trim() || join(homedir(), '.tauri', 'prime-updater.key')
  const passwordFile = env.PRIME_UPDATER_PASSWORD_FILE?.trim() || join(dirname(keyFile), 'prime-updater.password')
  const envPrivateKey = env.TAURI_SIGNING_PRIVATE_KEY?.trim()
  const privateKey = envPrivateKey || readTrimmedFile(keyFile)
  const password = env.TAURI_SIGNING_PRIVATE_KEY_PASSWORD ?? readTrimmedFile(passwordFile) ?? ''
  const tauriConfig = JSON.parse(readFileSync(join(root, 'src-tauri', 'tauri.conf.json'), 'utf8'))
  const publicKey = env.PRIME_UPDATER_PUBLIC_KEY?.trim() || tauriConfig.plugins?.updater?.pubkey?.trim() || ''
  const publicKeyFile = `${keyFile}.pub`
  const publicKeyFromFile = !envPrivateKey && existsSync(keyFile) ? readTrimmedFile(publicKeyFile) : undefined
  const endpoint =
    env.PRIME_UPDATER_ENDPOINT?.trim() ||
    tauriConfig.plugins?.updater?.endpoints?.[0]?.trim() ||
    DEFAULT_UPDATER_ENDPOINT
  return {
    keyFile,
    passwordFile,
    privateKey,
    password,
    publicKey,
    publicKeyFromFile,
    endpoint
  }
}

export function verifyUpdaterKeyPair({ root, privateKey, password, publicKey, publicKeyFromFile, tauriCliPath }) {
  if (!publicKey) {
    throw new Error('Updater public key is missing: set PRIME_UPDATER_PUBLIC_KEY or plugins.updater.pubkey')
  }
  const configuredPublicKey = parseMinisignPublicKey(publicKey)
  if (publicKeyFromFile) {
    const filePublicKey = parseMinisignPublicKey(publicKeyFromFile)
    if (filePublicKey.keyId !== configuredPublicKey.keyId) {
      throw new Error(
        `Updater key mismatch: local public key (${filePublicKey.keyId}) does not match configured public key (${configuredPublicKey.keyId}). Update desktop/src-tauri/tauri.conf.json plugins.updater.pubkey or set PRIME_UPDATER_PUBLIC_KEY.`
      )
    }
  }
  if (tauriCliPath && existsSync(tauriCliPath)) {
    if (!privateKey) {
      throw new Error('Updater signing private key is missing')
    }
    const tempDirectory = mkdtempSync(join(tmpdir(), 'prime-updater-key-check-'))
    try {
      const sampleFile = join(tempDirectory, 'updater-check.bin')
      const sampleBytes = Buffer.from('prime-updater-key-check')
      writeFileSync(sampleFile, sampleBytes)
      const signEnvironment = {
        ...process.env,
        TAURI_SIGNING_PRIVATE_KEY: privateKey,
        TAURI_SIGNING_PRIVATE_KEY_PASSWORD: password ?? ''
      }
      delete signEnvironment.TAURI_SIGNING_PRIVATE_KEY_PATH
      const result = spawnSync(
        process.execPath,
        [tauriCliPath, 'signer', 'sign', '--app-version', '0.0.0', sampleFile],
        {
          cwd: root || dirname(tauriCliPath),
          encoding: 'utf8',
          env: signEnvironment
        }
      )
      if (result.error) throw result.error
      if (result.status !== 0) {
        const detail = (result.stderr || result.stdout || '').trim()
        throw new Error(detail ? `Updater signing check failed: ${detail}` : `Updater signing check failed with exit code ${result.status}`)
      }
      const signaturePath = `${sampleFile}.sig`
      if (!existsSync(signaturePath)) {
        throw new Error('Updater signing check did not produce a signature file')
      }
      const signatureText = readFileSync(signaturePath, 'utf8').trim()
      verifyPublicKeyMatchesSignature(publicKey, signatureText, sampleBytes)
    } finally {
      rmSync(tempDirectory, { recursive: true, force: true })
    }
  }
  return { keyId: configuredPublicKey.keyId }
}

const isMainModule = process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)

if (isMainModule) {
  try {
    if (!process.argv.includes('--check')) {
      throw new Error('Usage: node updater-key.mjs --check')
    }
    const desktopRoot = fileURLToPath(new URL('..', import.meta.url))
    const keyConfig = resolveUpdaterKeyConfig(desktopRoot)
    if (!keyConfig.privateKey) {
      throw new Error(`Updater signing key not found. Put it in ${keyConfig.keyFile} or set TAURI_SIGNING_PRIVATE_KEY`)
    }
    const candidateCli = join(desktopRoot, 'node_modules', '@tauri-apps', 'cli', 'tauri.js')
    const verified = verifyUpdaterKeyPair({
      root: desktopRoot,
      privateKey: keyConfig.privateKey,
      password: keyConfig.password,
      publicKey: keyConfig.publicKey,
      publicKeyFromFile: keyConfig.publicKeyFromFile,
      tauriCliPath: existsSync(candidateCli) ? candidateCli : undefined
    })
    console.log(`Updater key verified: ${verified.keyId}`)
  } catch (error) {
    console.error(error?.message || String(error))
    process.exit(1)
  }
}
