export const remoteEditUiEn: Record<string, string> = {
  'remote.ui.openWithLocal': 'Open with local program',
  'remote.ui.openInEditor': 'Edit',
  'remote.ui.syncNow': 'Sync now',
  'remote.ui.stopSync': 'Stop syncing',
  'remote.ui.discardLocal': 'Discard local changes',

  'remote.ui.state.watching': 'Waiting for local changes',
  'remote.ui.state.syncing': 'Syncing',
  'remote.ui.state.synced': 'Synced',
  'remote.ui.state.conflict': 'Conflict',
  'remote.ui.state.error': 'Error',
  'remote.ui.state.stopped': 'Stopped',
  'remote.ui.stateLabel': 'Sync status: {state}',

  'remote.ui.conflict.title': 'The remote file changed',
  'remote.ui.conflict.description':
    'The remote content differs from what it was when you started editing. Overwrite writes back only when the remote still matches the content below; if the remote changed again, this dialog appears again instead of silently overwriting newer content.',
  'remote.ui.conflict.reload': 'Reload',
  'remote.ui.conflict.reloadHint': 'Discard local changes and read the latest content from the remote again',
  'remote.ui.conflict.overwrite': 'Overwrite',
  'remote.ui.conflict.overwriteHint': 'Write local content back to the remote',
  'remote.ui.conflict.cancel': 'Cancel',
  'remote.ui.conflict.cancelHint': 'Keep the current state and decide later',
  'remote.ui.conflict.externalNotice':
    'This edit was made in an external editor. The local temp file cannot be read across processes, so only the remote version is shown and no diff is available.',
  'remote.ui.conflict.diffTitle': 'Remote content ↔ local changes',
  'remote.ui.conflict.diffIdentical': 'Both sides are identical',
  'remote.ui.conflict.diffSummary': '{added} lines added / {removed} lines removed',
  'remote.ui.conflict.diffUnavailable':
    'The remote content could not be read (binary or incomplete), so no diff is shown.',
  'remote.ui.conflict.repeated':
    'The remote changed again while you were deciding, so nothing was written. Select again.',

  'remote.ui.editor.loading': 'Loading…',
  'remote.ui.editor.save': 'Save',
  'remote.ui.editor.close': 'Close',
  'remote.ui.editor.refresh': 'Refresh',
  'remote.ui.editor.refreshHint':
    'Refresh: discard local changes not yet synced to the remote and read the latest content again. Discarded changes cannot be recovered.',
  'remote.ui.editor.dirty': 'Unsaved changes',
  'remote.ui.editor.clean': 'No unsaved changes',
  'remote.ui.editor.saved': 'Saved to remote host',
  'remote.ui.editor.unsupportedBinary':
    'This is a binary file and cannot be edited inside the app. Use “Open with local program” instead.',
  'remote.ui.editor.unsupportedTruncated':
    'The file exceeds the in-app read limit (8 MiB) and was not read completely. Use “Open with local program” instead.',
  'remote.ui.editor.unsupportedTooLarge':
    'The file exceeds the in-app edit size limit. Use “Open with local program” instead.',
  'remote.ui.editor.fallback': 'The embedded editor could not load, so a plain-text editor is used instead.',
  'remote.ui.editor.discardTitle': 'Discard local changes',
  'remote.ui.editor.discardMessage':
    'The editor content is restored to the current remote content; unsaved changes are lost.',
  'remote.ui.editor.discardConfirm': 'Discard',
  'remote.ui.editor.closeDirtyTitle': 'Discard changes before closing?',
  'remote.ui.editor.closeDirtyMessage':
    'Changes in this window are lost when it closes; the remote file is not modified.',
  'remote.ui.editor.closeDirtyConfirm': 'Discard and close',

  'remote.ui.stop.title': 'Stop syncing',
  'remote.ui.stop.message':
    'Qterm stops watching for local changes and deletes the local temp file; changes not yet synced to the remote are lost.',
  'remote.ui.stop.confirm': 'Stop and delete',

  'remote.ui.toast.openedLocal': 'Opened with a local program: {path}',
  'remote.ui.toast.openFailed': 'Could not open: {reason}',
  'remote.ui.toast.synced': 'Synced to the remote host',
  'remote.ui.toast.syncFailed': 'Could not sync: {reason}',
  'remote.ui.toast.stopped': 'Syncing stopped and the local temp file was deleted',
  'remote.ui.toast.stopFailed': 'Could not stop syncing: {reason}',
  'remote.ui.toast.saveFailed': 'Could not save: {reason}',
}
