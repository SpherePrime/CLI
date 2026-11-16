import { generateKeyPairSync, randomBytes, sign } from 'node:crypto'
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { afterEach, describe, expect, it } from 'vitest'
import {
  parseMinisignPublicKey,
  parseMinisignSignature,
  resolveUpdaterKeyConfig,
  verifyPublicKeyMatchesSignature,
  verifyUpdaterKeyPair
} from '../../scripts/updater-key.mjs'

const OFFICIAL_PUBLIC_KEY =
  'dW50cnVzdGVkIGNvbW1lbnQ6IG1pbmlzaWduIHB1YmxpYyBrZXk6IDVEODlBMUIxRjU3REJBRjcKUldUM3VuMzFzYUdKWGRtMWR2NXZqeFBJSThOU2ZwOHBvYkpmb0Nnb3Vaa2dYRldMRG5zcmZzZnAK'
const PREVIOUS_MISMATCHED_PUBLIC_KEY =
  'dW50cnVzdGVkIGNvbW1lbnQ6IG1pbmlzaWduIHB1YmxpYyBrZXk6IDExRDg2RDgxMjJFQUYyRjYKUldUMjh1b2lnVzNZRWQrSGREV2hEUW1jMmU1MURZbGl0WHV1S1hDT2xjYld4Y1V4eVpaMzFBbysK'

function formatKeyId(keyIdBytes) {
  return Buffer.from(keyIdBytes).reverse().toString('hex').toUpperCase()
}

function encodeTestPublicKey(keyIdBytes, publicKeyObject) {
  const spki = publicKeyObject.export({ format: 'der', type: 'spki' })
  const rawPublicKey = spki.subarray(spki.length - 32)
  const payload = Buffer.concat([Buffer.from('Ed', 'ascii'), keyIdBytes, rawPublicKey])
  const box = `untrusted comment: minisign public key: ${formatKeyId(keyIdBytes)}\n${payload.toString('base64')}\n`
  return Buffer.from(box, 'utf8').toString('base64')
}

function encodeTestSignature(keyIdBytes, privateKeyObject, payloadBytes, algorithm = 'Ed') {
  const rawSignature = sign(null, payloadBytes, privateKeyObject)
  const payload = Buffer.concat([Buffer.from(algorithm, 'ascii'), keyIdBytes, rawSignature])
  const box = [
    'untrusted comment: signature from tauri secret key',
    payload.toString('base64'),
    'trusted comment: timestamp:0\tfile:test.bin',
    Buffer.alloc(64).toString('base64')
  ].join('\n')
  return Buffer.from(`${box}\n`, 'utf8').toString('base64')
}

const temporaryDirectories = []

afterEach(() => {
  while (temporaryDirectories.length > 0) {
    rmSync(temporaryDirectories.pop(), { recursive: true, force: true })
  }
})

describe('updater key verification', () => {
  it('extracts expected key IDs from minisign public keys', () => {
    const official = parseMinisignPublicKey(OFFICIAL_PUBLIC_KEY)
    expect(official.algorithm).toBe('Ed')
    expect(official.keyId).toBe('5D89A1B1F57DBAF7')
    expect(official.publicKey).toHaveLength(32)

    const mismatched = parseMinisignPublicKey(PREVIOUS_MISMATCHED_PUBLIC_KEY)
    expect(mismatched.algorithm).toBe('Ed')
    expect(mismatched.keyId).toBe('11D86D8122EAF2F6')
    expect(mismatched.publicKey).toHaveLength(32)
  })

  it('rejects malformed minisign public keys and signatures', () => {
    expect(() => parseMinisignPublicKey('')).toThrow(/empty/i)
    expect(() => parseMinisignPublicKey('not!valid')).toThrow(/malformed base64/i)
    expect(() => parseMinisignSignature(OFFICIAL_PUBLIC_KEY)).toThrow(/payload length/i)
  })

  it('verifies matching Ed25519 keypair and signature and rejects mismatches', () => {
    const keyId = randomBytes(8)
    const otherKeyId = Buffer.from(keyId)
    otherKeyId[0] ^= 0xff

    const { publicKey, privateKey } = generateKeyPairSync('ed25519')
    const otherKeyPair = generateKeyPairSync('ed25519')
    const payload = Buffer.from('prime-desktop-installer-payload')

    const encodedPublicKey = encodeTestPublicKey(keyId, publicKey)
    const encodedSignature = encodeTestSignature(keyId, privateKey, payload)
    const parsedSignature = parseMinisignSignature(encodedSignature)
    expect(parsedSignature.keyId).toBe(formatKeyId(keyId))

    expect(verifyPublicKeyMatchesSignature(encodedPublicKey, encodedSignature, payload)).toEqual({
      keyId: formatKeyId(keyId),
      algorithm: 'Ed'
    })

    const differentKeyIdSignature = encodeTestSignature(otherKeyId, privateKey, payload)
    expect(() => verifyPublicKeyMatchesSignature(encodedPublicKey, differentKeyIdSignature, payload)).toThrow(
      new RegExp(`${formatKeyId(keyId)}.*${formatKeyId(otherKeyId)}`)
    )

    const wrongKeySignature = encodeTestSignature(keyId, otherKeyPair.privateKey, payload)
    expect(() => verifyPublicKeyMatchesSignature(encodedPublicKey, wrongKeySignature, payload)).toThrow(
      /verification failed/i
    )
  })

  it('embeds official key 5D89A1B1F57DBAF7 in desktop/src-tauri/tauri.conf.json', () => {
    const tauriConfigPath = fileURLToPath(new URL('../../src-tauri/tauri.conf.json', import.meta.url))
    const tauriConfig = JSON.parse(readFileSync(tauriConfigPath, 'utf8'))
    const parsed = parseMinisignPublicKey(tauriConfig.plugins?.updater?.pubkey)
    expect(parsed.keyId).toBe('5D89A1B1F57DBAF7')
  })

  it('catches local .pub key mismatch before building', () => {
    const tempRoot = mkdtempSync(join(tmpdir(), 'prime-updater-config-'))
    temporaryDirectories.push(tempRoot)
    mkdirSync(join(tempRoot, 'src-tauri'), { recursive: true })
    writeFileSync(
      join(tempRoot, 'src-tauri', 'tauri.conf.json'),
      JSON.stringify({
        plugins: {
          updater: {
            pubkey: OFFICIAL_PUBLIC_KEY,
            endpoints: ['https://github.com/SpherePrime/CLI/releases/latest/download/latest.json']
          }
        }
      })
    )
    const keyPath = join(tempRoot, 'prime-updater.key')
    writeFileSync(keyPath, 'secret')
    writeFileSync(`${keyPath}.pub`, PREVIOUS_MISMATCHED_PUBLIC_KEY)

    const config = resolveUpdaterKeyConfig(tempRoot, {
      PRIME_UPDATER_KEY_FILE: keyPath
    })
    expect(config.publicKeyFromFile).toBe(PREVIOUS_MISMATCHED_PUBLIC_KEY)
    expect(() => verifyUpdaterKeyPair({ root: tempRoot, ...config })).toThrow(
      /11D86D8122EAF2F6.*5D89A1B1F57DBAF7/
    )
  })
})
