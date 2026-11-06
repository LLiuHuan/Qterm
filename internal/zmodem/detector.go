package zmodem

import (
	"bytes"
	"regexp"
)

var zmodemDetect = regexp.MustCompile(`(?s)\x2A\x2A\x18\x42\x30[\x30\x31]`)

type Detector struct {
	buffer bytes.Buffer
}

func NewDetector() *Detector {
	return &Detector{}
}

func (d *Detector) Write(p []byte) (int, error) {
	return d.buffer.Write(p)
}

func (d *Detector) Reset() {
	d.buffer.Reset()
}

func (d *Detector) Detect() (detected bool, cleanData []byte) {
	data := d.buffer.Bytes()
	loc := zmodemDetect.FindIndex(data)
	if loc == nil {
		return false, data
	}

	_ = loc
	return true, data
}
