import { remoteErrorSource } from '../../../../shared/remoteErrors'
import type { RemoteStatus } from '../../../../shared/remote'
const statuses: Record<RemoteStatus, string> = {
  disconnected: 'Отключён',
  connecting: 'Подключение',
  verifying: 'Проверка ключа',
  preparing: 'Подготовка Prime',
  connected: 'Подключён',
  disconnecting: 'Отключение',
  error: 'Ошибка подключения'
}
export function statusLabel(status: RemoteStatus) {
  return statuses[status]
}
export function remoteError(error: unknown, t: (text: string) => string) {
  return t(remoteErrorSource(error) ?? 'Не удалось выполнить операцию на сервере.')
}

const progress: Record<string, string> = {
  'remote.preparingRuntime': 'Проверка Prime на сервере',
  'remote.uploadingRuntime': 'Загрузка Prime на сервер',
  'remote.startRuntime': 'Запуск Prime на сервере'
}
export function remoteProgress(message: string, t: (text: string) => string) {
  return t(progress[message] ?? message)
}
