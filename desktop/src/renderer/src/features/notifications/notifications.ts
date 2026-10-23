export type ToastAction = 'providers'

export type Toast = {
  id: number
  title: string
  message: string
  action?: ToastAction
}

type Listener = (toasts: Toast[]) => void

const listeners = new Set<Listener>()
let toasts: Toast[] = []
let nextId = 1

function publish() {
  for (const listener of listeners) listener(toasts)
}

export function subscribeToasts(listener: Listener) {
  listeners.add(listener)
  listener(toasts)
  return () => {
    listeners.delete(listener)
  }
}

export function dismissToast(id: number) {
  toasts = toasts.filter((toast) => toast.id !== id)
  publish()
}

const capability = /Native capability '([^']+)' is not implemented yet/
const providerHint = /provider|model|api key|apikey|провайдер|модел|unauthorized|401|403/i

export function describeError(error: unknown): Omit<Toast, 'id'> {
  const raw = String(
    (error as { message?: string })?.message ?? (typeof error === 'string' ? error : error) ?? ''
  )
    .replace(/^Error invoking remote method '[^']+': Error: /, '')
    .replace(/^Error: /, '')
    .trim()
  const missing = capability.exec(raw)
  if (missing)
    return {
      title: 'Функция недоступна',
      message: `Действие «${missing[1]}» пока не поддерживается в этой версии.`
    }
  if (/Engine status 5\d\d|ENGINE_/.test(raw))
    return { title: 'Ошибка движка Prime', message: raw }
  if (providerHint.test(raw))
    return { title: 'Проблема с провайдером', message: raw, action: 'providers' }
  return { title: 'Что-то пошло не так', message: raw || 'Неизвестная ошибка' }
}

export function notifyError(error: unknown) {
  const toast = { id: nextId++, ...describeError(error) }
  if (!toast.message) return
  const duplicate = toasts.find(
    (item) => item.title === toast.title && item.message === toast.message
  )
  toasts = [...toasts.filter((item) => item !== duplicate), toast].slice(-4)
  publish()
}
