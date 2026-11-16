import type { ResourceSnapshot } from '../../services/sessions'
import { formatByteRate } from '../../lib/format'
import { resourceNetworks } from './resourceSelection'

export function networkMetrics(snapshot: ResourceSnapshot | null | undefined) {
  const validRate = (value: number) => (Number.isFinite(value) && value > 0 ? value : 0)
  const network = resourceNetworks(snapshot?.networks)[0]
  const receiveRate = validRate(network?.receiveRateBytes ?? 0)
  const transmitRate = validRate(network?.transmitRateBytes ?? 0)
  return {
    receiveRate,
    transmitRate,
    receiveText: formatByteRate(receiveRate),
    transmitText: formatByteRate(transmitRate),
  }
}
