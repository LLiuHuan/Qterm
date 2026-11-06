interface WailsWindow extends Window {
  _wails?: {
    environment?: {
      OS?: string
    }
  }
}

const recoveryPackageFilter = {
  DisplayName: 'Qterm 恢复包 (*.qtp)',
  Pattern: '*.qtp',
}

const backgroundImageFilters = [
  { DisplayName: '图片 (*.png;*.jpg;*.jpeg;*.webp;*.gif)', Pattern: '*.png;*.jpg;*.jpeg;*.webp;*.gif' },
]

export function hasNativeFileDialogs(): boolean {
  return Boolean((window as WailsWindow)._wails?.environment?.OS)
}

export async function pickRecoveryPackage(): Promise<string> {
  const { Dialogs } = await import('@wailsio/runtime')
  const selected = await Dialogs.OpenFile({
    CanChooseFiles: true,
    CanChooseDirectories: false,
    AllowsMultipleSelection: false,
    CanCreateDirectories: false,
    Filters: [recoveryPackageFilter],
    Title: '选择 QTPK 恢复包',
    ButtonText: '打开',
    Detached: false,
  })
  return Array.isArray(selected) ? (selected[0] ?? '') : selected || ''
}

export async function pickRecoveryPackageDestination(): Promise<string> {
  const { Dialogs } = await import('@wailsio/runtime')
  return (
    (await Dialogs.SaveFile({
      Filename: 'qterm-recovery.qtp',
      CanChooseDirectories: false,
      CanChooseFiles: true,
      CanCreateDirectories: true,
      Filters: [recoveryPackageFilter],
      Title: '保存 QTPK 恢复包',
      ButtonText: '导出',
      Detached: false,
    })) || ''
  )
}

export async function pickBackgroundImage(): Promise<string> {
  const { Dialogs } = await import('@wailsio/runtime')
  const selected = await Dialogs.OpenFile({
    CanChooseFiles: true,
    CanChooseDirectories: false,
    AllowsMultipleSelection: false,
    CanCreateDirectories: false,
    Filters: backgroundImageFilters,
    Title: '选择背景图片',
    ButtonText: '选择',
    Detached: false,
  })
  return Array.isArray(selected) ? (selected[0] ?? '') : selected || ''
}

export async function pickLocalShellExecutable(): Promise<string> {
  const { Dialogs } = await import('@wailsio/runtime')
  const selected = await Dialogs.OpenFile({
    CanChooseFiles: true,
    CanChooseDirectories: false,
    AllowsMultipleSelection: false,
    CanCreateDirectories: false,
    Title: '选择默认 Shell 可执行文件',
    ButtonText: '选择',
    Detached: false,
  })
  return Array.isArray(selected) ? (selected[0] ?? '') : selected || ''
}

export async function pickLocalShellDirectory(): Promise<string> {
  const { Dialogs } = await import('@wailsio/runtime')
  const selected = await Dialogs.OpenFile({
    CanChooseFiles: false,
    CanChooseDirectories: true,
    AllowsMultipleSelection: false,
    CanCreateDirectories: false,
    Title: '选择本机终端启动目录',
    ButtonText: '选择',
    Detached: false,
  })
  return Array.isArray(selected) ? (selected[0] ?? '') : selected || ''
}
