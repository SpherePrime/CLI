import { expect, it } from 'vitest'
import { validateExternalUrl } from '../../src/main/ipc/security'
import { operationRoute, redact } from '../../src/main/prime/operations'
it('rejects privileged URL schemes', () => {
  expect(() => validateExternalUrl('file:///C:/Windows/system32/cmd.exe')).toThrow()
  expect(() => validateExternalUrl('javascript:alert(1)')).toThrow()
  expect(validateExternalUrl('https://example.com/auth')).toBe('https://example.com/auth')
})
it('keeps usage but removes credentials from nested config', () => {
  expect(
    redact({
      prompt_tokens: 123,
      providers: { test: { api_key: 'secret', oauth: { access_token: 'secret' }, name: 'Test' } }
    })
  ).toEqual({ prompt_tokens: 123, providers: { test: { name: 'Test' } } })
})
it('encodes session identifiers and refuses arbitrary routes', () => {
  expect(operationRoute('messages', { sessionId: '../config' })[1]).toBe(
    'sessions/..%2Fconfig/messages'
  )
  expect(() => operationRoute('anything' as never)).toThrow()
})
