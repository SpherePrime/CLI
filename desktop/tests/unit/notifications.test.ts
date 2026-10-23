import { describe, expect, it } from 'vitest'
import {
  describeError,
  dismissToast,
  notifyError,
  subscribeToasts,
  type Toast
} from '../../src/renderer/src/features/notifications/notifications'

describe('error notifications', () => {
  it('turns missing native capabilities into a readable message', () => {
    expect(describeError(new Error("Native capability 'trashEntry' is not implemented yet"))).toEqual({
      title: 'Функция недоступна',
      message: 'Действие «trashEntry» пока не поддерживается в этой версии.'
    })
  })

  it('offers provider settings for provider failures', () => {
    expect(describeError('Provider returned 401 Unauthorized').action).toBe('providers')
    expect(describeError('disk full').title).toBe('Что-то пошло не так')
  })

  it('keeps one toast per message and at most four', () => {
    let current: Toast[] = []
    const stop = subscribeToasts((toasts) => (current = toasts))
    notifyError('same')
    notifyError('same')
    expect(current).toHaveLength(1)
    for (const index of [1, 2, 3, 4, 5]) notifyError(`error ${index}`)
    expect(current).toHaveLength(4)
    for (const toast of current) dismissToast(toast.id)
    expect(current).toHaveLength(0)
    stop()
  })
})
