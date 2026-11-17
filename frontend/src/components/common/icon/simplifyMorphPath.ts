export function simplifyMorphPath(d: string): string {
  if (!d.includes('L') || /[^MLZ0-9eE+.,\s-]/.test(d)) return d

  return (d.match(/M[^M]*/g) ?? []).map(simplifySubpath).join('')
}

const toleranceSquared = 0.03 ** 2
const numberPattern = /[-+]?(?:\d*\.?\d+)(?:[eE][-+]?\d+)?/g

function simplifySubpath(d: string): string {
  const coordinates = (d.match(numberPattern) ?? []).map(Number)
  const count = coordinates.length / 2
  if (count <= 2 || !Number.isInteger(count) || coordinates.some((value) => !Number.isFinite(value))) return d

  const keep = new Uint8Array(count)
  keep[0] = keep[count - 1] = 1

  const pending: [number, number][] = [[0, count - 1]]
  while (pending.length) {
    const [first, last] = pending.pop()!
    const x = coordinates[first * 2]!
    const y = coordinates[first * 2 + 1]!
    const dx = coordinates[last * 2]! - x
    const dy = coordinates[last * 2 + 1]! - y
    const lengthSquared = dx * dx + dy * dy
    let farthest = -1
    let maxDistance = toleranceSquared
    for (let i = first + 1; i < last; i++) {
      const px = coordinates[i * 2]! - x
      const py = coordinates[i * 2 + 1]! - y
      const t = lengthSquared ? Math.max(0, Math.min(1, (px * dx + py * dy) / lengthSquared)) : 0
      const distance = (px - t * dx) ** 2 + (py - t * dy) ** 2
      if (distance > maxDistance) {
        maxDistance = distance
        farthest = i
      }
    }
    if (farthest < 0) continue
    keep[farthest] = 1
    pending.push([first, farthest], [farthest, last])
  }

  let result = `M${coordinates[0]} ${coordinates[1]}`
  for (let i = 1; i < count; i++) {
    if (keep[i]) result += `L${coordinates[i * 2]} ${coordinates[i * 2 + 1]}`
  }
  return result + (d.trimEnd().endsWith('Z') ? 'Z' : '')
}
