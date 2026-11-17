import { describe, expect, it } from 'vitest'
import { translate } from '../../src/shared/localization'
import { remoteErrors, remoteErrorSource } from '../../src/shared/remoteErrors'
import { english } from '../../src/shared/translations'

describe('remoteErrors', () => {
  it('resolves all new SSH connection stage error codes', () => {
    const cases: Array<[string, string]> = [
      [
        'remote.handshakeFailed',
        'Не удалось согласовать параметры SSH-подключения с сервером.'
      ],
      ['remote.authFailed', 'Не удалось войти на сервер. Проверьте логин, пароль или ключ.'],
      ['remote.agentUnavailable', 'SSH-агент недоступен или не содержит подходящий ключ.'],
      ['remote.sftpFailed', 'На сервере недоступна подсистема SFTP.'],
      [
        'remote.credentialsUnavailable',
        'Защищённое хранилище ОС недоступно. Пароль не сохранён.'
      ]
    ]

    for (const [code, expected] of cases) {
      expect(remoteErrorSource(code)).toBe(expected)
      expect(remoteErrorSource(new Error(`SSH error: ${code}`))).toBe(expected)
    }
  })

  it('resolves existing shared SSH contract error codes', () => {
    expect(remoteErrorSource('remote.connectionFailed')).toBe(
      'Не удалось подключиться. Проверьте адрес сервера и способ входа.'
    )
    expect(remoteErrorSource('remote.credentialsRequired')).toBe('Введите пароль для подключения.')
    expect(remoteErrorSource('remote.notDirectory')).toBe('Папка проекта на сервере не найдена.')
  })

  it('returns undefined for unknown errors', () => {
    expect(remoteErrorSource('remote.unknownCode')).toBeUndefined()
    expect(remoteErrorSource(new Error('Something unexpected happened'))).toBeUndefined()
    expect(remoteErrorSource('')).toBeUndefined()
    expect(remoteErrorSource(null)).toBeUndefined()
    expect(remoteErrorSource(undefined)).toBeUndefined()
  })

  it('provides English translations for every remote error message', () => {
    for (const source of Object.values(remoteErrors)) {
      expect(english[source]).toBeTruthy()
      expect(translate('en', source)).toBe(english[source])
      expect(translate('ru', source)).toBe(source)
    }
  })
})
