export interface PreviewableImageType {
  extension: string

  mimeType: string
}

export const PREVIEWABLE_IMAGE_TYPES: readonly PreviewableImageType[] = [
  { extension: 'png', mimeType: 'image/png' },
  { extension: 'jpg', mimeType: 'image/jpeg' },
  { extension: 'jpeg', mimeType: 'image/jpeg' },
  { extension: 'gif', mimeType: 'image/gif' },
  { extension: 'webp', mimeType: 'image/webp' },
  { extension: 'bmp', mimeType: 'image/bmp' },
  { extension: 'ico', mimeType: 'image/x-icon' },
  { extension: 'cur', mimeType: 'image/x-icon' },
  { extension: 'svg', mimeType: 'image/svg+xml' },
  { extension: 'avif', mimeType: 'image/avif' },
]

const MIME_BY_EXTENSION: Record<string, string> = Object.fromEntries(
  PREVIEWABLE_IMAGE_TYPES.map((item) => [item.extension, item.mimeType]),
)

export const PREVIEW_IMAGE_MIMES: readonly string[] = [
  'image/png',
  'image/jpeg',
  'image/gif',
  'image/webp',
  'image/bmp',
  'image/x-icon',
  'image/vnd.microsoft.icon',
  'image/svg+xml',
  'image/avif',
]

export function previewImageExtension(nameOrPath: string): string {
  const slash = Math.max(nameOrPath.lastIndexOf('/'), nameOrPath.lastIndexOf('\\'))
  const name = nameOrPath.slice(slash + 1).toLowerCase()
  const dot = name.lastIndexOf('.')
  if (dot <= 0 || dot === name.length - 1) return ''
  return name.slice(dot + 1)
}

export function previewImageMimeForName(nameOrPath: string): string | null {
  const extension = previewImageExtension(nameOrPath)
  if (!extension) return null
  return MIME_BY_EXTENSION[extension] ?? null
}

export function isPreviewableImageName(nameOrPath: string): boolean {
  return previewImageMimeForName(nameOrPath) !== null
}

export function isPreviewImageMime(mimeType: string): boolean {
  return PREVIEW_IMAGE_MIMES.includes(mimeType.trim().toLowerCase())
}
