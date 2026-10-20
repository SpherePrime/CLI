export const remoteErrors: Record<string, string> = {
  'remote.historyReadFailed':
    'Не удалось прочитать прежнюю историю SSH-проекта. Локальная история не заменена.',
  'remote.historySnapshotUnavailable':
    'Для переноса прежней истории нужен sqlite3 или Python 3 на сервере.',
  'remote.historySnapshotFailed':
    'Не удалось создать согласованную копию прежней истории. Исходные чаты сохранены.',
  'remote.historyInvalidDestination':
    'Не удалось безопасно сохранить историю SSH-проекта на этом компьютере.',
  'remote.secureStorageUnavailable': 'Защищённое хранилище ОС недоступно. Пароль не сохранён.',
  'remote.credentialsCorrupt': 'Хранилище SSH-паролей повреждено. Исходный файл сохранён.',
  'remote.credentialsUnreadable':
    'Не удалось расшифровать SSH-пароль. Введите его заново или удалите сохранённый пароль.',
  'remote.runtimeFailed': 'Prime на сервере остановился. Подключитесь снова.',
  'remote.runtimeUnavailable': 'Не удалось подготовить Prime на сервере.',
  'remote.unsupportedPlatform': 'Сервер должен работать на Linux x64 или ARM64.',
  'remote.tunnelDenied': 'Сервер запретил SSH-туннель. Проверьте AllowTcpForwarding.',
  'remote.corrupt':
    'Хранилище серверов повреждено. Изменения заблокированы, исходный файл сохранён.',
  'remote.invalidProfile': 'Проверьте адрес, порт, пользователя и абсолютный путь Linux.',
  'remote.connectionFailed': 'Не удалось подключиться. Проверьте адрес сервера и способ входа.',
  'remote.connectionLost': 'Соединение с сервером потеряно. Подключитесь снова.',
  'remote.credentialsRequired': 'Введите пароль для подключения.',
  'remote.invalidKey': 'Не удалось прочитать приватный ключ. Максимальный размер — 1 MiB.',
  'remote.staleChallenge': 'Отпечаток изменился. Проверьте сервер заново.',
  'remote.disconnectFirst': 'Отключите сервер перед изменением профиля.',
  'remote.alreadyConnecting': 'Подключение уже выполняется.',
  'remote.notDirectory': 'Папка проекта на сервере не найдена.',
  'remote.pathOutside': 'Путь выходит за пределы удалённого проекта.',
  'remote.revisionConflict': 'Файл изменился на сервере. Обновите его перед сохранением.',
  'remote.notConnected': 'Сервер отключён.',
  'remote.fileTooLarge': 'Файл больше 2 MiB.',
  'remote.binaryFile': 'Предпросмотр бинарного файла недоступен.',
  'remote.gitFailed': 'Не удалось прочитать Git на сервере.',
  'remote.commandTimeout': 'Сервер не ответил вовремя.',
  'remote.outputTooLarge': 'Ответ сервера превышает допустимый размер.'
}

export function remoteErrorSource(error: unknown) {
  const message = error instanceof Error ? error.message : String(error ?? '')
  const code = Object.keys(remoteErrors).find((key) => message.includes(key))
  return code ? remoteErrors[code] : undefined
}
