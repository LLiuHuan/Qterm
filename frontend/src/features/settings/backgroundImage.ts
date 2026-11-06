import { loadSettings, saveSettings } from './settingsStore'
import { translate } from '@/locales'

export async function readBackgroundImage(file: File, kind: 'light' | 'dark'): Promise<string> {
  if (!['image/png', 'image/jpeg', 'image/webp', 'image/gif'].includes(file.type)) {
    throw new Error(translate('background.core.invalidType'))
  }
  const url = URL.createObjectURL(file)
  try {
    const image = new Image()
    image.src = url
    await image.decode()
    const scale = Math.min(1, 1920 / Math.max(image.naturalWidth, image.naturalHeight))
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(image.naturalWidth * scale))
    canvas.height = Math.max(1, Math.round(image.naturalHeight * scale))
    const context = canvas.getContext('2d')
    if (!context) throw new Error(translate('background.core.processingFailed'))
    context.drawImage(image, 0, 0, canvas.width, canvas.height)
    const data = canvas.toDataURL('image/jpeg', 0.8)
    if (data.length > 1_500_000) throw new Error(translate('background.core.tooLarge'))
    const settings = loadSettings()
    const key = kind === 'light' ? 'lightBackgroundImage' : 'darkBackgroundImage'
    settings[key] = data

    saveSettings(settings)
    return data
  } finally {
    URL.revokeObjectURL(url)
  }
}
