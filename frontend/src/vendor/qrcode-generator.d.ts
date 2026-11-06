export interface QRCodeInstance {
  addData(data: string, mode?: string): void
  make(): void
  getModuleCount(): number
  isDark(row: number, col: number): boolean
  createSvgTag(options?: number | { cellSize?: number; margin?: number; scalable?: boolean }): string
}

export default function qrcode(typeNumber: number, errorCorrectionLevel: 'L' | 'M' | 'Q' | 'H'): QRCodeInstance
