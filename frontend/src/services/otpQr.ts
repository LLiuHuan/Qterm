export const OTP_QR_MAX_BYTES = 8 * 1024 * 1024
export const OTP_QR_MAX_PIXELS = 16_000_000

export interface ParsedOtpUri {
  type: 'totp' | 'hotp'
  name: string
  username: string
  secret: string
  algorithm: 'SHA1' | 'SHA256' | 'SHA512'
  digits: number
  period: number
  counter: number
}

export function parseOtpProvisioningUri(value: string): ParsedOtpUri {
  const uri = value.trim()
  let parsed: URL
  try {
    parsed = new URL(uri)
  } catch {
    throw new Error('invalid_otp_uri')
  }
  const type =
    parsed.protocol === 'otpauth:' && (parsed.hostname === 'totp' || parsed.hostname === 'hotp') ? parsed.hostname : ''
  if (!type) throw new Error('invalid_otp_uri')

  const secret = parsed.searchParams.get('secret')?.replace(/\s+/g, '').toUpperCase() ?? ''
  if (!secret || !/^[A-Z2-7]+=*$/.test(secret)) throw new Error('invalid_otp_uri')

  const label = (() => {
    try {
      return decodeURIComponent(parsed.pathname.replace(/^\//, ''))
    } catch {
      throw new Error('invalid_otp_uri')
    }
  })()
  const issuerParam = parsed.searchParams.get('issuer')?.trim() ?? ''
  const separator = label.indexOf(':')
  const labelIssuer = separator >= 0 ? label.slice(0, separator).trim() : ''
  const username = (separator >= 0 ? label.slice(separator + 1) : label).trim()
  const name = issuerParam || labelIssuer || username
  if (!name) throw new Error('invalid_otp_uri')

  const algorithmValue = (parsed.searchParams.get('algorithm') || 'SHA1').toUpperCase()
  const algorithm =
    algorithmValue === 'SHA256' || algorithmValue === 'SHA512'
      ? algorithmValue
      : algorithmValue === 'SHA1'
        ? 'SHA1'
        : ''
  if (!algorithm) throw new Error('invalid_otp_uri')

  const digits = Number(parsed.searchParams.get('digits') || 6)
  if (!Number.isInteger(digits) || digits < 4 || digits > 10) throw new Error('invalid_otp_uri')
  const period = Number(parsed.searchParams.get('period') || 30)
  const counter = Number(parsed.searchParams.get('counter') || 0)
  if (type === 'totp' && (!Number.isInteger(period) || period < 10 || period > 300)) {
    throw new Error('invalid_otp_uri')
  }
  if (type === 'hotp' && (!Number.isInteger(counter) || counter < 0 || counter > 999999999)) {
    throw new Error('invalid_otp_uri')
  }

  return {
    type,
    name,
    username,
    secret,
    algorithm,
    digits,
    period: type === 'totp' ? period : 30,
    counter: type === 'hotp' ? counter : 0,
  }
}

export async function decodeOtpQrImage(file: Blob): Promise<string> {
  if (file.size > OTP_QR_MAX_BYTES) throw new Error('otp_qr_image_too_large')

  const bitmap: ImageBitmap | HTMLImageElement =
    typeof globalThis.createImageBitmap === 'function' ? await createImageBitmap(file) : await loadImageFallback(file)
  try {
    const width = 'naturalWidth' in bitmap ? bitmap.naturalWidth : bitmap.width
    const height = 'naturalHeight' in bitmap ? bitmap.naturalHeight : bitmap.height
    const pixels = width * height
    if (pixels > OTP_QR_MAX_PIXELS) throw new Error('otp_qr_image_too_large')
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const context = canvas.getContext('2d', { willReadFrequently: true })
    if (!context) throw new Error('otp_qr_image_unreadable')
    context.drawImage(bitmap, 0, 0)
    const image = context.getImageData(0, 0, width, height)
    const { default: jsQR } = await import('jsqr')
    const result = jsQR(image.data, image.width, image.height)
    if (!result?.data) throw new Error('otp_qr_not_found')
    return result.data
  } finally {
    if ('close' in bitmap) bitmap.close()
  }
}

async function loadImageFallback(file: Blob): Promise<HTMLImageElement> {
  const url = URL.createObjectURL(file)
  try {
    const image = await new Promise<HTMLImageElement>((resolve, reject) => {
      const element = new Image()
      element.onload = () => resolve(element)
      element.onerror = () => reject(new Error('otp_qr_image_unreadable'))
      element.src = url
    })
    return image
  } finally {
    URL.revokeObjectURL(url)
  }
}
