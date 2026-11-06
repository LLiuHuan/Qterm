export const remoteEditUiRu: Record<string, string> = {
  'remote.ui.openWithLocal': 'Открыть в локальной программе',
  'remote.ui.openInEditor': 'Редактировать',
  'remote.ui.syncNow': 'Синхронизировать сейчас',
  'remote.ui.stopSync': 'Остановить синхронизацию',
  'remote.ui.discardLocal': 'Отменить локальные изменения',
  'remote.ui.state.watching': 'Ожидание локальных изменений',
  'remote.ui.state.syncing': 'Синхронизация…',
  'remote.ui.state.synced': 'Синхронизировано',
  'remote.ui.state.conflict': 'Конфликт',
  'remote.ui.state.error': 'Ошибка',
  'remote.ui.state.stopped': 'Остановлено',
  'remote.ui.stateLabel': 'Состояние синхронизации: {state}',
  'remote.ui.conflict.title': 'Удалённый файл изменён',
  'remote.ui.conflict.description':
    'Удалённое содержимое отличается от того, что было в начале редактирования. Перезапись выполняется только тогда, когда на удалённом хосте всё ещё находится содержимое, приведённое ниже; если оно снова изменится, этот диалог появится опять, а более новое содержимое не будет перезаписано молча.',
  'remote.ui.conflict.reload': 'Перезагрузить',
  'remote.ui.conflict.reloadHint':
    'Отменить локальные изменения и заново прочитать актуальное содержимое с удалённого хоста',
  'remote.ui.conflict.overwrite': 'Перезаписать',
  'remote.ui.conflict.overwriteHint': 'Записать локальное содержимое обратно на удалённый хост',
  'remote.ui.conflict.cancel': 'Отмена',
  'remote.ui.conflict.cancelHint': 'Оставить всё как есть и решить позже',
  'remote.ui.conflict.externalNotice':
    'Изменения внесены во внешнем редакторе. Локальный временный файл нельзя прочитать из другого процесса, поэтому показана только удалённая версия, а сравнение недоступно.',
  'remote.ui.conflict.diffTitle': 'Текущее содержимое на удалённом хосте ↔ локальные изменения',
  'remote.ui.conflict.diffIdentical': 'Содержимое совпадает',
  'remote.ui.conflict.diffSummary': 'Добавлено строк: {added}, удалено строк: {removed}',
  'remote.ui.conflict.diffUnavailable':
    'Не удалось прочитать удалённое содержимое (это двоичный файл или чтение неполное), поэтому сравнение не показано.',
  'remote.ui.conflict.repeated':
    'Пока вы принимали решение, удалённое содержимое снова изменилось, и запись не была выполнена. Сделайте выбор заново.',
  'remote.ui.editor.loading': 'Загрузка…',
  'remote.ui.editor.save': 'Сохранить',
  'remote.ui.editor.close': 'Закрыть',
  'remote.ui.editor.refresh': 'Обновить',
  'remote.ui.editor.refreshHint':
    'Обновление: локальные изменения, ещё не синхронизированные с удалённым хостом, будут отменены, а актуальное содержимое прочитано заново (отменённые изменения восстановить нельзя).',
  'remote.ui.editor.dirty': 'Есть несохранённые изменения',
  'remote.ui.editor.clean': 'Несохранённых изменений нет',
  'remote.ui.editor.saved': 'Сохранено на удалённом хосте',
  'remote.ui.editor.unsupportedBinary':
    'Это двоичный файл, его нельзя редактировать в приложении. Используйте «Открыть в локальной программе».',
  'remote.ui.editor.unsupportedTruncated':
    'Файл превышает лимит чтения для редактирования в приложении (8 MiB) и прочитан не полностью. Используйте «Открыть в локальной программе».',
  'remote.ui.editor.unsupportedTooLarge':
    'Файл превышает лимит размера для редактирования в приложении. Используйте «Открыть в локальной программе».',
  'remote.ui.editor.fallback': 'Встроенный редактор не загрузился, поэтому используется редактор обычного текста.',
  'remote.ui.editor.discardTitle': 'Отменить локальные изменения',
  'remote.ui.editor.discardMessage':
    'Содержимое редактора вернётся к текущему содержимому на удалённом хосте, а несохранённые изменения будут потеряны.',
  'remote.ui.editor.discardConfirm': 'Отменить изменения',
  'remote.ui.editor.closeDirtyTitle': 'Отменить изменения перед закрытием?',
  'remote.ui.editor.closeDirtyMessage':
    'После закрытия изменения в этом окне будут потеряны, а удалённый файл не изменится.',
  'remote.ui.editor.closeDirtyConfirm': 'Отменить и закрыть',
  'remote.ui.stop.title': 'Остановить синхронизацию',
  'remote.ui.stop.message':
    'Отслеживание локальных изменений прекратится, локальный временный файл будет удалён; изменения, ещё не синхронизированные с удалённым хостом, будут потеряны.',
  'remote.ui.stop.confirm': 'Остановить и удалить',
  'remote.ui.toast.openedLocal': 'Открыто в локальной программе: {path}',
  'remote.ui.toast.openFailed': 'Не удалось открыть: {reason}',
  'remote.ui.toast.synced': 'Синхронизировано с удалённым хостом',
  'remote.ui.toast.syncFailed': 'Не удалось синхронизировать: {reason}',
  'remote.ui.toast.stopped': 'Синхронизация остановлена, локальный временный файл удалён',
  'remote.ui.toast.stopFailed': 'Не удалось остановить синхронизацию: {reason}',
  'remote.ui.toast.saveFailed': 'Не удалось сохранить: {reason}',
}
