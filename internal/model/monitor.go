package model

type ProcessInfo struct {
	PID    int     `json:"pid"`
	PPID   int     `json:"ppid"`
	User   string  `json:"user"`
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	RSSKB  int64   `json:"rssKb"`

	VSZKB int64 `json:"vszKb"`

	TTY   string `json:"tty"`
	State string `json:"state"`

	Priority int `json:"priority"`

	Nice int `json:"nice"`

	Elapsed string `json:"elapsed"`

	CPUTime string `json:"cpuTime"`
	Command string `json:"command"`

	StartTime string `json:"startTime"`

	MetricsAvailable bool `json:"metricsAvailable"`

	CPUAvailable bool `json:"cpuAvailable"`
}

type ProcessSnapshot struct {
	Processes []ProcessInfo `json:"processes"`

	UpdatedAt int64 `json:"updatedAt"`
}

type AcceleratorKind string

const (
	AcceleratorGPU AcceleratorKind = "gpu"
	AcceleratorNPU AcceleratorKind = "npu"
)

type AcceleratorInfo struct {
	Index int             `json:"index"`
	Name  string          `json:"name"`
	Kind  AcceleratorKind `json:"kind"`

	Utilization float64 `json:"utilization"`

	MemoryUsedMB  float64 `json:"memoryUsedMb"`
	MemoryTotalMB float64 `json:"memoryTotalMb"`

	TemperatureC float64 `json:"temperatureC"`
}

type AcceleratorSnapshot struct {
	Devices   []AcceleratorInfo `json:"devices"`
	UpdatedAt int64             `json:"updatedAt"`
}
