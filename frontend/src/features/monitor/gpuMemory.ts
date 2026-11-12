export interface GpuMemoryInput {
  kind: string
  memoryUsedMb: number
  memoryTotalMb: number
}

export interface GpuMemorySummary {
  usedMb: number

  totalMb: number

  usedPercent: number
}

export function gpuMemorySummary(devices: GpuMemoryInput[] | null | undefined): GpuMemorySummary | null {
  if (!devices?.length) return null
  const gpus = devices.filter((device) => device.kind === 'gpu')
  if (!gpus.length) return null

  let usedMb = 0
  let totalMb = 0
  for (const gpu of gpus) {
    usedMb += finiteOrZero(gpu.memoryUsedMb)
    totalMb += finiteOrZero(gpu.memoryTotalMb)
  }
  return {
    usedMb,
    totalMb,
    usedPercent: totalMb > 0 ? Math.min(100, (usedMb / totalMb) * 100) : Number.NaN,
  }
}

export function formatGpuMemory(mb: number): string {
  const value = finiteOrZero(mb)
  if (value <= 0) return '0 MB'
  return value >= 1024 ? `${(value / 1024).toFixed(1)} GB` : `${Math.round(value)} MB`
}

function finiteOrZero(value: number): number {
  if (!Number.isFinite(value) || value < 0) return 0
  return value
}
