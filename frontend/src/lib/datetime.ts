export const DATETIME_PLACEHOLDER = '-'

export type DateTimeInput = string | number | Date | null | undefined

export interface FormatDateTimeOptions {
  placeholder?: string
}

function pad2(value: number): string {
  return String(value).padStart(2, '0')
}

function toLocalDate(value: DateTimeInput): Date | null {
  if (value === null || value === undefined) return null
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value
  }
  if (typeof value === 'number') {
    if (!Number.isFinite(value) || value === 0) return null
    const fromNumber = new Date(value)
    return Number.isNaN(fromNumber.getTime()) ? null : fromNumber
  }
  const trimmed = value.trim()
  if (!trimmed) return null

  if (trimmed.startsWith('0001-01-01')) return null
  const fromString = new Date(trimmed)
  return Number.isNaN(fromString.getTime()) ? null : fromString
}

export function formatDateTime(value: DateTimeInput, options: FormatDateTimeOptions = {}): string {
  const { placeholder = DATETIME_PLACEHOLDER } = options
  const date = toLocalDate(value)
  if (!date) return placeholder
  const ymd = `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`
  const hms = `${pad2(date.getHours())}:${pad2(date.getMinutes())}:${pad2(date.getSeconds())}`
  return `${ymd} ${hms}`
}
