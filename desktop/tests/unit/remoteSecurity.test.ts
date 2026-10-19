import { describe, it, expect } from 'vitest'
import { validateProfile, shellQuote, remotePath } from '../../src/main/remote/security'
describe('remote boundaries', () => {
  it('accepts Linux roots and strips undeclared secrets', () => {
    const result = validateProfile({
      name: 'Build',
      host: 'host',
      port: 22,
      user: 'dev',
      root: '/srv/app',
      auth: 'agent',
      runtime: 'bundled',
      password: 'secret'
    })
    expect(result.root).toBe('/srv/app')
    expect(JSON.stringify(result)).not.toContain('secret')
  })
  it('rejects traversal and absolute paths', () => {
    for (const path of ['../secret', '/etc/passwd', 'a\\b', 'a/../../secret'])
      expect(() => remotePath('/srv/app', path)).toThrow()
    expect(remotePath('/srv/app', 'src/a.ts')).toBe('/srv/app/src/a.ts')
  })
  it('quotes hostile shell arguments', () => {
    expect(shellQuote("a'b; $(id)")).toBe("'a'\\''b; $(id)'")
  })
  it('rejects unsupported profiles', () => {
    expect(() =>
      validateProfile({
        name: 'x',
        host: 'h',
        user: 'u',
        root: 'C:\\app',
        port: 22,
        auth: 'agent',
        runtime: 'bundled'
      })
    ).toThrow()
  })
})
