function hasNativeClipboard(): boolean {
  return Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
}

export async function readClipboardText(): Promise<string> {
  if (hasNativeClipboard()) {
    const { Clipboard } = await import('@wailsio/runtime')
    return Clipboard.Text()
  }
  return navigator.clipboard.readText()
}

export async function writeClipboardText(text: string): Promise<void> {
  if (hasNativeClipboard()) {
    const { Clipboard } = await import('@wailsio/runtime')
    await Clipboard.SetText(text)
    return
  }
  await navigator.clipboard.writeText(text)
}
