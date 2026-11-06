package tunnel

import (
	"context"
	"strings"
	"sync"
	"time"

	"changeme/internal/model"
)

const (
	resourceDiskRefreshInterval = 30 * time.Second

	resourceDiskProbeTimeout = 3 * time.Second

	resourceSampleWindowMax = 5 * time.Minute
)

const linuxResourceCountersCommand = `
export LC_ALL=C
[ -r /proc/stat ] && [ -r /proc/meminfo ] && printf 'metrics_supported=1\n'
awk '/^cpu[0-9]* /{print "cpu=" $1 "," ($2+$3+$4+$5+$6+$7+$8+$9) "," ($5+$6)}' /proc/stat
awk 'NR>2 {gsub(":","",$1); if($1!="lo") print "net=" $1 "," $2 "," $10}' /proc/net/dev
awk '/^MemTotal:/{print "mem_total=" $2} /^MemAvailable:/{print "mem_available=" $2} /^Cached:/{print "mem_cached=" $2}' /proc/meminfo
awk '{print "uptime=" $1}' /proc/uptime
awk '{print "load1=" $1; print "load5=" $2; print "load15=" $3}' /proc/loadavg
`

const linuxResourceStaticCommand = `
export LC_ALL=C
host=$(hostname 2>/dev/null)
[ -n "$host" ] || host=unknown
printf 'host=%s\n' "$host"
arch=$(uname -m 2>/dev/null)
[ -n "$arch" ] || arch=unknown
printf 'arch=%s\n' "$arch"
release_file=/etc/os-release
[ -r "$release_file" ] || release_file=/usr/lib/os-release
[ -r "$release_file" ] || release_file=/system/etc/os-release
if [ -r "$release_file" ]; then
  awk -F= '
    function release_value(line) {
      sub(/^[^=]*=/, "", line)
      gsub(/^"|"$/, "", line)
      return line
    }
    $1 == "PRETTY_NAME" && pretty == "" { pretty=release_value($0) }
    $1 == "ID" && id == "" { id=release_value($0) }
    $1 == "NAME" && name == "" { name=release_value($0) }
    END {
      if (pretty != "") print "os_pretty=" pretty
      if (id != "") print "os_id=" id
      if (name != "") print "os_name=" name
    }
  ' "$release_file"
else
  os=$(uname -s 2>/dev/null)
  [ -n "$os" ] || os=unknown
  printf 'os=%s\n' "$os"
fi
getconf _NPROCESSORS_ONLN 2>/dev/null | sed 's/^/cpu=/'
`

const linuxResourceDiskCommand = `
export LC_ALL=C
df -PT -k 2>/dev/null | awk 'NR>1 {gsub(/%/,"",$6);print "disk="$7","$6","$4","$3","$2}'
`

type resourceCPUCounter struct {
	total uint64
	idle  uint64
}

type resourceNetCounter struct {
	rx uint64
	tx uint64
}

type resourceStaticInfo struct {
	hostname string
	arch     string
	os       string
	cores    int
}

type resourceSampler struct {
	mu           sync.Mutex
	static       resourceStaticInfo
	staticLoaded bool
	cpu          map[string]resourceCPUCounter
	net          map[string]resourceNetCounter
	sampledAt    time.Time
	disks        []model.DiskResource
	disksAt      time.Time
}

func (c *Connection) ReadResources(ctx context.Context) (ResourceSnapshot, error) {
	if c == nil {
		return ResourceSnapshot{}, ErrRemoteOSUnsupported
	}
	c.resources.mu.Lock()
	defer c.resources.mu.Unlock()

	if !c.resources.staticLoaded {
		out, err := c.runProbe(ctx, linuxResourceStaticCommand, resourceProbeOutputLimit)
		if err != nil {
			return ResourceSnapshot{}, err
		}
		info, err := parseResourceStatic(out)
		if err != nil {
			return ResourceSnapshot{}, err
		}
		c.resources.static = info
		c.resources.staticLoaded = true
	}
	static := c.resources.static

	out, err := c.runProbe(ctx, linuxResourceCountersCommand, resourceProbeOutputLimit)
	if err != nil {
		return ResourceSnapshot{}, err
	}
	counters, err := parseResourceCounters(out)
	if err != nil {
		return ResourceSnapshot{}, err
	}

	now := time.Now()
	window := time.Duration(0)
	if !c.resources.sampledAt.IsZero() {
		if elapsed := now.Sub(c.resources.sampledAt); elapsed > 0 && elapsed <= resourceSampleWindowMax {
			window = elapsed
		}
	}

	result := ResourceSnapshot{
		MetricsSupported:     counters.metricsSupported,
		Hostname:             static.hostname,
		OS:                   static.os,
		Architecture:         static.arch,
		CPUCores:             static.cores,
		MemoryTotalBytes:     counters.memoryTotalBytes,
		MemoryAvailableBytes: counters.memoryAvailableBytes,
		MemoryCachedBytes:    counters.memoryCachedBytes,
		UptimeSeconds:        counters.uptimeSeconds,
		LoadAverage1:         counters.load1,
		LoadAverage5:         counters.load5,
		LoadAverage15:        counters.load15,
		SampleWindowMS:       window.Milliseconds(),
	}
	result.CPUUsagePercent, result.CPUPerCore = diffCPU(c.resources.cpu, counters.cpu, window)
	result.Networks = diffNetworks(c.resources.net, counters.net, window)

	if len(c.resources.disks) == 0 || now.Sub(c.resources.disksAt) >= resourceDiskRefreshInterval {
		diskCtx, cancel := context.WithTimeout(ctx, resourceDiskProbeTimeout)
		diskOut, diskErr := c.runProbe(diskCtx, linuxResourceDiskCommand, resourceProbeOutputLimit)
		cancel()
		if diskErr == nil {
			c.resources.disks = parseResourceDisks(diskOut)
			c.resources.disksAt = now
		}
	}
	result.Disks = append([]model.DiskResource(nil), c.resources.disks...)

	c.resources.cpu = counters.cpu
	c.resources.net = counters.net
	c.resources.sampledAt = now

	result.MetricsSupported = resourceMetricsSupported(result.MetricsSupported, result.MemoryTotalBytes, len(counters.cpu))
	return finalizeResourceSnapshot(result), nil
}

func resourceMetricsSupported(marker bool, memoryTotalBytes uint64, cpuCount int) bool {
	return marker && memoryTotalBytes > 0 && cpuCount > 0
}

type resourceCounters struct {
	metricsSupported     bool
	cpu                  map[string]resourceCPUCounter
	net                  map[string]resourceNetCounter
	memoryTotalBytes     uint64
	memoryAvailableBytes uint64
	uptimeSeconds        uint64
	memoryCachedBytes    uint64
	load1                *float64
	load5                *float64
	load15               *float64
}

func parseResourceCounters(out string) (resourceCounters, error) {
	counters := resourceCounters{cpu: map[string]resourceCPUCounter{}, net: map[string]resourceNetCounter{}}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "metrics_supported":
			counters.metricsSupported = value == "1"
		case "cpu":
			parts := strings.Split(value, ",")
			if len(parts) == 3 {
				counters.cpu[strings.TrimSpace(parts[0])] = resourceCPUCounter{
					total: parseUint(parts[1]),
					idle:  parseUint(parts[2]),
				}
			}
		case "net":
			parts := strings.Split(value, ",")
			if len(parts) == 3 {
				counters.net[strings.TrimSpace(parts[0])] = resourceNetCounter{
					rx: parseUint(parts[1]),
					tx: parseUint(parts[2]),
				}
			}
		case "mem_total":
			counters.memoryTotalBytes = parseUint(value) * 1024
		case "mem_available":
			counters.memoryAvailableBytes = parseUint(value) * 1024
		case "mem_cached":
			counters.memoryCachedBytes = parseUint(value) * 1024
		case "uptime":
			counters.uptimeSeconds = uint64(parseFloat(value))
		case "load1":
			counters.load1 = parseLoad(value)
		case "load5":
			counters.load5 = parseLoad(value)
		case "load15":
			counters.load15 = parseLoad(value)
		}
	}
	return counters, nil
}

func parseResourceStatic(out string) (resourceStaticInfo, error) {
	var info resourceStaticInfo
	var osFallback, osPretty, osID, osName string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "host":
			info.hostname = value
		case "arch":
			info.arch = value
		case "os":
			osFallback = cleanOSReleaseValue(value)
		case "os_pretty", "PRETTY_NAME":
			osPretty = cleanOSReleaseValue(value)
		case "os_id", "ID":
			osID = cleanOSReleaseValue(value)
		case "os_name", "NAME":
			osName = cleanOSReleaseValue(value)
		case "cpu":
			info.cores = int(parseUint(value))
		}
	}
	if info.hostname == "" {
		return resourceStaticInfo{}, ErrRemoteOSUnsupported
	}
	info.os = firstNonEmpty(osPretty, osID, osName, osFallback)
	return info, nil
}

func parseResourceDisks(out string) []model.DiskResource {
	var disks []model.DiskResource
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok || key != "disk" {
			continue
		}
		parts := strings.SplitN(value, ",", 5)
		if len(parts) != 5 {
			continue
		}
		disks = append(disks, model.DiskResource{
			Mount:          parts[0],
			UsedPercent:    clampPercent(parseFloat(parts[1])),
			UsedBytes:      parseUint(parts[2]) * 1024,
			TotalBytes:     parseUint(parts[3]) * 1024,
			FileSystemType: strings.TrimSpace(parts[4]),
		})
	}
	return disks
}

func diffCPU(previous map[string]resourceCPUCounter, current map[string]resourceCPUCounter, window time.Duration) (float64, []float64) {
	if window <= 0 || len(previous) == 0 {
		return 0, []float64{}
	}
	usage := func(name string) (float64, bool) {
		prev, ok := previous[name]
		if !ok {
			return 0, false
		}
		now, ok := current[name]
		if !ok {
			return 0, false
		}
		totalDelta := now.total - prev.total
		if now.total < prev.total || totalDelta == 0 {
			return 0, false
		}
		idleDelta := now.idle - prev.idle
		if now.idle < prev.idle {
			idleDelta = 0
		}
		if idleDelta > totalDelta {
			idleDelta = totalDelta
		}
		return clampPercent((1 - float64(idleDelta)/float64(totalDelta)) * 100), true
	}

	perCore := make([]float64, 0, len(current))
	coreSum := 0.0
	coreCount := 0
	for name := range current {
		if name == "cpu" {
			continue
		}
		value, ok := usage(name)
		if !ok {
			continue
		}
		perCore = append(perCore, value)
		coreSum += value
		coreCount++
	}
	aggregate, ok := usage("cpu")
	if !ok && coreCount > 0 {
		aggregate = clampPercent(coreSum / float64(coreCount))
	}
	return aggregate, perCore
}

func diffNetworks(previous map[string]resourceNetCounter, current map[string]resourceNetCounter, window time.Duration) []model.NetworkResource {
	seconds := window.Seconds()
	networks := make([]model.NetworkResource, 0, len(current))
	for name, now := range current {
		network := model.NetworkResource{
			Name:          name,
			ReceiveBytes:  now.rx,
			TransmitBytes: now.tx,
		}
		if seconds > 0 {
			if prev, ok := previous[name]; ok && now.rx >= prev.rx && now.tx >= prev.tx {
				network.ReceiveRateBytes = uint64(float64(now.rx-prev.rx) / seconds)
				network.TransmitRateBytes = uint64(float64(now.tx-prev.tx) / seconds)
			}
		}
		networks = append(networks, network)
	}
	return networks
}

func parseLoad(value string) *float64 {
	load := parseFloat(value)
	if load < 0 || load != load {
		return nil
	}
	return &load
}
