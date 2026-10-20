import { safeStorage } from 'electron'
import { RemoteCredentialStore } from './credentials'

export function createCredentialVault(path: string) {
  return new RemoteCredentialStore(path, {
    available: () =>
      safeStorage.isEncryptionAvailable() &&
      (process.platform !== 'linux' || safeStorage.getSelectedStorageBackend() !== 'basic_text'),
    encrypt: (text) => safeStorage.encryptString(text),
    decrypt: (bytes) => safeStorage.decryptString(bytes)
  })
}
