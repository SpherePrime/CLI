export type RemoteProfile = {
  id: string
  name: string
  host: string
  port: number
  user: string
  root: string
  auth: 'agent' | 'key' | 'password'
  keyPath?: string
  runtime: 'bundled' | 'existing'
  runtimePath?: string
}
export type RemoteProfileInput = Omit<RemoteProfile, 'id'> & { id?: string }
export type RemoteCredentials = { password?: string; passphrase?: string }
export type RemoteStatus =
  | 'disconnected'
  | 'connecting'
  | 'verifying'
  | 'preparing'
  | 'connected'
  | 'disconnecting'
  | 'error'
export type RemoteHostChallenge = { fingerprint: string; previous?: string }
export type RemoteProfileState = RemoteProfile & {
  hasSavedCredentials?: boolean
  status: RemoteStatus
  challenge?: RemoteHostChallenge
  error?: string
  progress?: { message: string; percent?: number }
}
export type RemoteSnapshot = {
  profiles: RemoteProfileState[]
  error?: string
  credentialStorageAvailable?: boolean
  credentialError?: string
}
export type RemoteResponse = RemoteSnapshot
export type RemoteRequest =
  | { action: 'list' }
  | { action: 'save'; input: RemoteProfileInput }
  | { action: 'remove' | 'disconnect' | 'forgetCredentials'; id: string }
  | { action: 'connect'; id: string; credentials?: RemoteCredentials }
  | { action: 'trust'; id: string; fingerprint: string }
export type RemoteApi = (request: RemoteRequest) => Promise<RemoteResponse>
