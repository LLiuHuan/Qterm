package tunnel

import "changeme/internal/model"

func finalizeResourceSnapshot(result ResourceSnapshot) ResourceSnapshot {
	var memoryUsedBytes uint64
	if result.MemoryTotalBytes > 0 && result.MemoryAvailableBytes <= result.MemoryTotalBytes {
		memoryUsedBytes = result.MemoryTotalBytes - result.MemoryAvailableBytes
		result.MemoryUsagePercent = float64(memoryUsedBytes) / float64(result.MemoryTotalBytes) * 100
	}
	result.MemoryUsedText = model.FormatBytes(memoryUsedBytes)
	result.MemoryTotalText = model.FormatBytes(result.MemoryTotalBytes)
	result.MemoryAvailableText = model.FormatBytes(result.MemoryAvailableBytes)
	result.MemoryCachedText = model.FormatBytes(result.MemoryCachedBytes)
	for i := range result.Networks {
		result.Networks[i].ReceiveText = model.FormatBytes(result.Networks[i].ReceiveRateBytes)
		result.Networks[i].TransmitText = model.FormatBytes(result.Networks[i].TransmitRateBytes)
	}
	for i := range result.Disks {
		result.Disks[i].UsedText = model.FormatBytes(result.Disks[i].UsedBytes)
		result.Disks[i].TotalText = model.FormatBytes(result.Disks[i].TotalBytes)
	}

	if result.CPUPerCore == nil {
		result.CPUPerCore = []float64{}
	}
	if result.Networks == nil {
		result.Networks = []model.NetworkResource{}
	}
	if result.Disks == nil {
		result.Disks = []model.DiskResource{}
	}
	return result
}
