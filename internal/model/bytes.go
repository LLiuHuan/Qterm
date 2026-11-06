package model

import (
	"fmt"
	"math"
	"strings"
)

const (
	bytesBase  = 1024.0
	bytesScale = 100.0
)

var bytesUnits = [...]string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}

func FormatBytes(value uint64) string {
	if value < 10 {
		return fmt.Sprintf("%d B", value)
	}

	unitIndex, scaled := pickUnit(float64(value))
	scaled = round2(scaled)
	if scaled >= bytesBase && unitIndex < len(bytesUnits)-1 {
		unitIndex++
		scaled = round2(scaled / bytesBase)
	}

	return formatDecimal(scaled) + " " + bytesUnits[unitIndex]
}

func pickUnit(v float64) (int, float64) {
	idx := 0
	for v >= bytesBase && idx < len(bytesUnits)-1 {
		v /= bytesBase
		idx++
	}
	return idx, v
}

func round2(v float64) float64 {
	return math.Round(v*bytesScale) / bytesScale
}

func formatDecimal(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
