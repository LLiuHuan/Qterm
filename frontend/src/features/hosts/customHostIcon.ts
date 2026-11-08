export const CUSTOM_HOST_ICON_PREFIX = 'data:image/png;base64,'
export const MAX_CUSTOM_HOST_ICON_LENGTH = 64 * 1024
export const MAX_CUSTOM_HOST_ICON_DIMENSION = 64
export const MAX_CUSTOM_HOST_ICON_PIXELS = 16_000_000

const MAX_SOURCE_IMAGE_BYTES = 8 * 1024 * 1024
const ACCEPTED_SOURCE_TYPES = new Set(['image/png', 'image/jpeg', 'image/webp', 'image/gif'])

export type CustomHostIconReadErrorCode =
  'unsupported-type' | 'source-too-large' | 'image-too-large' | 'decode-failed' | 'encode-failed'

export class CustomHostIconReadError extends Error {
  readonly code: CustomHostIconReadErrorCode

  constructor(code: CustomHostIconReadErrorCode) {
    super(code)
    this.name = 'CustomHostIconReadError'
    this.code = code
  }
}

export function isCustomHostIcon(value: unknown): boolean {
  if (typeof value !== 'string' || value.length > MAX_CUSTOM_HOST_ICON_LENGTH) return false
  if (!value.startsWith(CUSTOM_HOST_ICON_PREFIX)) return false
  const payload = value.slice(CUSTOM_HOST_ICON_PREFIX.length)
  if (!payload || !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(payload)) return false
  try {
    const binary = atob(payload)
    const signature = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]
    return signature.every((byte, index) => binary.charCodeAt(index) === byte)
  } catch {
    return false
  }
}

export function customHostIconSrc(value: unknown): string | null {
  return isCustomHostIcon(value) && typeof value === 'string' ? value : null
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new CustomHostIconReadError('decode-failed'))
    reader.onload = () => {
      if (typeof reader.result === 'string') resolve(reader.result)
      else reject(new CustomHostIconReadError('decode-failed'))
    }
    reader.readAsDataURL(file)
  })
}

function decodeImage(dataUrl: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new CustomHostIconReadError('decode-failed'))
    image.src = dataUrl
  })
}

export async function readCustomHostIcon(file: File): Promise<string> {
  if (!ACCEPTED_SOURCE_TYPES.has(file.type)) throw new CustomHostIconReadError('unsupported-type')
  if (file.size > MAX_SOURCE_IMAGE_BYTES) throw new CustomHostIconReadError('source-too-large')

  const source = await readFileAsDataUrl(file)
  const image = await decodeImage(source)
  const width = image.naturalWidth || image.width
  const height = image.naturalHeight || image.height
  if (!width || !height) throw new CustomHostIconReadError('decode-failed')
  if (width * height > MAX_CUSTOM_HOST_ICON_PIXELS) {
    throw new CustomHostIconReadError('image-too-large')
  }

  const scale = Math.min(1, MAX_CUSTOM_HOST_ICON_DIMENSION / Math.max(width, height))
  const canvas = document.createElement('canvas')
  canvas.width = Math.max(1, Math.round(width * scale))
  canvas.height = Math.max(1, Math.round(height * scale))
  const context = canvas.getContext('2d')
  if (!context) throw new CustomHostIconReadError('encode-failed')
  context.clearRect(0, 0, canvas.width, canvas.height)
  context.drawImage(image, 0, 0, canvas.width, canvas.height)

  let result: string
  try {
    result = canvas.toDataURL('image/png')
  } catch {
    throw new CustomHostIconReadError('encode-failed')
  }
  if (!isCustomHostIcon(result)) throw new CustomHostIconReadError('encode-failed')
  return result
}
