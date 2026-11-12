package tunnel

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"changeme/internal/model"
)

const (
	processProbeOutputLimit = 512 * 1024
	processProbeTimeout     = 6 * time.Second
	acceleratorProbeTimeout = 4 * time.Second
	acceleratorOutputLimit  = 32 * 1024
)

const processesWithStartCommand = `LC_ALL=C ps -eo pid,ppid,user,pcpu,pmem,rss,vsz,tty,stat,pri,ni,etime,time,lstart,args 2>/dev/null`
const processesCommand = `LC_ALL=C ps -eo pid,ppid,user,pcpu,pmem,rss,stat,args 2>/dev/null`
const processesBasicCommand = `LC_ALL=C ps -eo pid,user,args 2>/dev/null`

const processesBusyBoxCommand = `LC_ALL=C ps -eo pid,ppid,user,rss,vsz,tty,stat,nice,etime,time,args 2>/dev/null
awk '/^MemTotal:/{print "mem_total=" $2}' /proc/meminfo 2>/dev/null`

const processCPUTicksCommand = `awk '{ rest=$0; sub(/^.*\) /,"",rest); n=split(rest,a," "); if (n >= 13) print $1, a[12], a[13] }' /proc/[0-9]*/stat 2>/dev/null`

const processClockTicks = 100.0

var errProcessListUnavailable = errors.New("tunnel: remote process list is unavailable")

const nvidiaSMICommand = `nvidia-smi --query-gpu=index,utilization.gpu,memory.used,memory.total,temperature.gpu,name --format=csv,noheader,nounits 2>/dev/null`

const npuSMICommand = `npu-smi info -t common 2>/dev/null`

func (c *Connection) ReadProcesses(ctx context.Context) (model.ProcessSnapshot, error) {
	if c == nil {
		return model.ProcessSnapshot{}, ErrRemoteOSUnsupported
	}
	probeCtx, cancel := context.WithTimeout(ctx, processProbeTimeout)
	defer cancel()

	probes := []struct {
		command string
		parse   func(string) []model.ProcessInfo
	}{
		{processesWithStartCommand, parseProcessListWithStart},
		{processesCommand, parseProcessList},
		{processesBusyBoxCommand, parseBusyBoxProcessList},
		{processesBasicCommand, parseBasicProcessList},
	}
	var lastErr error
	processes := make([]model.ProcessInfo, 0, 64)
	for _, probe := range probes {
		output, err := c.runProbe(probeCtx, probe.command, processProbeOutputLimit)
		if err != nil {
			lastErr = err
			continue
		}
		processes = probe.parse(output)
		if len(processes) > 0 {
			break
		}
	}
	if len(processes) == 0 {
		if lastErr != nil {
			return model.ProcessSnapshot{}, lastErr
		}
		return model.ProcessSnapshot{}, errProcessListUnavailable
	}

	if !processes[0].CPUAvailable {
		c.applyProcessCPUSample(probeCtx, processes)
	}
	sort.SliceStable(processes, func(i, j int) bool {
		if processes[i].CPU == processes[j].CPU {
			return processes[i].RSSKB > processes[j].RSSKB
		}
		return processes[i].CPU > processes[j].CPU
	})
	return model.ProcessSnapshot{Processes: processes, UpdatedAt: time.Now().UnixMilli()}, nil
}

func (c *Connection) ReadAccelerators(ctx context.Context) (model.AcceleratorSnapshot, error) {
	if c == nil {
		return model.AcceleratorSnapshot{}, ErrRemoteOSUnsupported
	}
	probeCtx, cancel := context.WithTimeout(ctx, acceleratorProbeTimeout)
	defer cancel()

	devices := make([]model.AcceleratorInfo, 0, 4)
	if output, err := c.runProbe(probeCtx, nvidiaSMICommand, acceleratorOutputLimit); err == nil {
		devices = append(devices, parseNvidiaSMI(output)...)
	}
	if output, err := c.runProbe(probeCtx, npuSMICommand, acceleratorOutputLimit); err == nil {
		devices = append(devices, parseNpuSMI(output)...)
	}
	return model.AcceleratorSnapshot{Devices: devices, UpdatedAt: time.Now().UnixMilli()}, nil
}

func parseProcessList(output string) []model.ProcessInfo {
	return parseProcessListFields(output, false)
}

func parseProcessListWithStart(output string) []model.ProcessInfo {
	return parseProcessListFields(output, true)
}

func parseProcessListFields(output string, withStart bool) []model.ProcessInfo {
	result := make([]model.ProcessInfo, 0, 64)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)

		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		minimum := 8
		if withStart {

			minimum = 19
		}
		if len(fields) < minimum {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		cpu, _ := strconv.ParseFloat(fields[3], 64)
		memory, _ := strconv.ParseFloat(fields[4], 64)
		rss, _ := strconv.ParseInt(fields[5], 10, 64)
		commandIndex := 7
		startTime := ""
		vsz := int64(0)
		tty := ""
		state := fields[6]
		priority := 0
		nice := 0
		elapsed := ""
		cpuTime := ""
		if withStart {
			vsz, _ = strconv.ParseInt(fields[6], 10, 64)
			tty = fields[7]
			state = fields[8]
			priority, _ = strconv.Atoi(fields[9])
			nice, _ = strconv.Atoi(fields[10])
			elapsed = fields[11]
			cpuTime = fields[12]
			commandIndex = 18

			if started, parseErr := time.Parse("Mon Jan _2 15:04:05 2006", strings.Join(fields[13:18], " ")); parseErr == nil {
				startTime = started.Format("2006-01-02 15:04:05")
			}
		}
		command := normalizePSCommand(strings.Join(fields[commandIndex:], " "))
		result = append(result, model.ProcessInfo{
			PID:              pid,
			PPID:             ppid,
			User:             fields[2],
			CPU:              cpu,
			Memory:           memory,
			RSSKB:            rss,
			VSZKB:            vsz,
			TTY:              tty,
			State:            state,
			Priority:         priority,
			Nice:             nice,
			Elapsed:          elapsed,
			CPUTime:          cpuTime,
			Command:          command,
			StartTime:        startTime,
			MetricsAvailable: true,
			CPUAvailable:     true,
		})
	}
	return result
}

func normalizePSCommand(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	}
	return trimmed
}

type processCPUSampler struct {
	mu     sync.Mutex
	prev   map[int]uint64
	prevAt time.Time
}

func (c *Connection) applyProcessCPUSample(ctx context.Context, processes []model.ProcessInfo) {
	output, err := c.runProbe(ctx, processCPUTicksCommand, processProbeOutputLimit)
	if err != nil {
		return
	}
	ticks := parseProcessCPUTicks(output)
	if len(ticks) == 0 {
		return
	}
	now := time.Now()
	c.processCPU.mu.Lock()
	prev, prevAt := c.processCPU.prev, c.processCPU.prevAt
	c.processCPU.prev, c.processCPU.prevAt = ticks, now
	c.processCPU.mu.Unlock()
	applyProcessCPUTicks(processes, prev, prevAt, ticks, now)
}

func parseProcessCPUTicks(output string) map[int]uint64 {
	ticks := make(map[int]uint64, 64)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		utime, utimeErr := strconv.ParseUint(fields[1], 10, 64)
		stime, stimeErr := strconv.ParseUint(fields[2], 10, 64)

		if utimeErr != nil || stimeErr != nil {
			continue
		}
		ticks[pid] = utime + stime
	}
	return ticks
}

func applyProcessCPUTicks(
	processes []model.ProcessInfo,
	prev map[int]uint64,
	prevAt time.Time,
	current map[int]uint64,
	now time.Time,
) {
	elapsed := now.Sub(prevAt).Seconds()
	for index := range processes {
		processes[index].CPUAvailable = true
		tickCount, found := current[processes[index].PID]
		if !found {
			processes[index].CPU = 0
			continue
		}
		before, seen := prev[processes[index].PID]

		if prev == nil || !seen || tickCount < before || elapsed <= 0 {
			processes[index].CPU = 0
			continue
		}
		processes[index].CPU = float64(tickCount-before) / processClockTicks / elapsed * 100
	}
}

func parseBusyBoxProcessList(output string) []model.ProcessInfo {
	result := make([]model.ProcessInfo, 0, 64)
	memTotalKB := int64(0)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if value, found := strings.CutPrefix(trimmed, "mem_total="); found {
			memTotalKB, _ = strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			continue
		}
		fields := strings.Fields(trimmed)

		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		if len(fields) < 11 {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		rss, _ := strconv.ParseInt(fields[3], 10, 64)
		vsz, _ := strconv.ParseInt(fields[4], 10, 64)
		nice, _ := strconv.Atoi(fields[7])
		result = append(result, model.ProcessInfo{
			PID:     pid,
			PPID:    ppid,
			User:    fields[2],
			RSSKB:   rss,
			VSZKB:   vsz,
			TTY:     fields[5],
			State:   fields[6],
			Nice:    nice,
			Elapsed: fields[8],
			CPUTime: fields[9],
			Command: normalizePSCommand(strings.Join(fields[10:], " ")),

			MetricsAvailable: true,

			CPUAvailable: false,
		})
	}
	if memTotalKB > 0 {
		for index := range result {
			result[index].Memory = float64(result[index].RSSKB) / float64(memTotalKB) * 100
		}
	}
	return result
}

func parseBasicProcessList(output string) []model.ProcessInfo {
	result := make([]model.ProcessInfo, 0, 64)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		result = append(result, model.ProcessInfo{
			PID:     pid,
			User:    fields[1],
			Command: normalizePSCommand(strings.Join(fields[2:], " ")),
		})
	}
	return result
}

func parseNvidiaSMI(output string) []model.AcceleratorInfo {
	devices := make([]model.AcceleratorInfo, 0, 2)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 6)
		if len(parts) < 5 {
			continue
		}
		index, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		name := "NVIDIA GPU"
		if len(parts) == 6 && strings.TrimSpace(parts[5]) != "" {
			name = strings.TrimSpace(parts[5])
		}
		devices = append(devices, model.AcceleratorInfo{
			Index:         index,
			Name:          name,
			Kind:          model.AcceleratorGPU,
			Utilization:   parseFloatField(parts[1]),
			MemoryUsedMB:  parseFloatField(parts[2]),
			MemoryTotalMB: parseFloatField(parts[3]),
			TemperatureC:  parseFloatField(parts[4]),
		})
	}
	return devices
}

func parseNpuSMI(output string) []model.AcceleratorInfo {
	devices := make([]model.AcceleratorInfo, 0, 2)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.Contains(trimmed, "%") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		index, err := strconv.Atoi(strings.TrimSuffix(fields[0], ":"))
		if err != nil {
			continue
		}
		utilization := 0.0
		for _, field := range fields {
			if strings.HasSuffix(field, "%") {
				utilization = parseFloatField(strings.TrimSuffix(field, "%"))
				break
			}
		}
		if utilization == 0 {
			continue
		}
		devices = append(devices, model.AcceleratorInfo{
			Index:       index,
			Name:        "NPU",
			Kind:        model.AcceleratorNPU,
			Utilization: utilization,
		})
	}
	return devices
}

func parseFloatField(value string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}
