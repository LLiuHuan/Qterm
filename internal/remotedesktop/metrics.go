package remotedesktop

import (
	"sync"
	"time"
)

const rateWindow = 2 * time.Second

type rateSample struct {
	at    time.Time
	value uint64
}

type eventRate struct {
	window  time.Duration
	now     func() time.Time
	samples []rateSample
}

func newEventRate() *eventRate {
	return &eventRate{window: rateWindow, now: time.Now}
}

func (r *eventRate) observe(value uint64) {
	if r == nil {
		return
	}
	now := r.now()
	r.samples = append(r.samples, rateSample{at: now, value: value})
	cutoff := now.Add(-r.window)
	keep := 0
	for keep < len(r.samples) && r.samples[keep].at.Before(cutoff) {
		keep++
	}
	if keep > 0 {

		r.samples = r.samples[keep:]
	}
}

func (r *eventRate) rate() float64 {
	if r == nil || len(r.samples) < 2 {
		return 0
	}
	oldest := r.samples[0]
	newest := r.samples[len(r.samples)-1]
	elapsed := newest.at.Sub(oldest.at).Seconds()
	if elapsed <= 0 || newest.value <= oldest.value {
		return 0
	}
	return float64(newest.value-oldest.value) / elapsed
}

type streamMetrics struct {
	mu sync.Mutex

	batchesSent    uint64
	keyFramesSent  uint64
	bytesSent      uint64
	rectsSent      uint64
	decodeDrops    uint64
	encodeFailures uint64
	lastSnapshotMS float64
	lastEncodeMS   float64
	lastSendMS     float64
	lastBatchRects int
	lastBatchBytes int

	inFlight      bool
	inFlightBytes int
	clients       int

	slowClient uint64

	renderedBatches uint64
	lastRenderMS    float64

	serverRate *eventRate
	renderRate *eventRate

	lastSample time.Time
}

func newStreamMetrics() *streamMetrics {
	return &streamMetrics{serverRate: newEventRate(), renderRate: newEventRate()}
}

type metricsSnapshot struct {
	batchesSent     uint64
	keyFramesSent   uint64
	bytesSent       uint64
	rectsSent       uint64
	decodeDrops     uint64
	encodeFailures  uint64
	lastSnapshotMS  float64
	lastEncodeMS    float64
	lastSendMS      float64
	lastBatchRects  int
	lastBatchBytes  int
	inFlight        bool
	inFlightBytes   int
	clients         int
	slowClient      uint64
	renderedBatches uint64
	lastRenderMS    float64
	serverFPS       float64
	renderFPS       float64
}

func (m *streamMetrics) recordSnapshot(rects int, took time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastBatchRects = rects
	m.lastSnapshotMS = took.Seconds() * 1000
}

func (m *streamMetrics) recordEncode(rects int, payloadBytes int, took time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastEncodeMS = took.Seconds() * 1000
	m.lastBatchRects = rects
	m.lastBatchBytes = payloadBytes
}

func (m *streamMetrics) recordBatchSent(keyFrame bool, rects int, bytes int, took time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchesSent++
	m.rectsSent += uint64(rects)
	m.bytesSent += uint64(bytes)
	if keyFrame {
		m.keyFramesSent++
	}
	m.lastSendMS = took.Seconds() * 1000
	m.inFlight = true
	m.inFlightBytes = bytes
}

func (m *streamMetrics) recordAck(rendered bool, renderMS float64, serverFrames uint64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inFlight = false
	m.inFlightBytes = 0
	if rendered {
		m.renderedBatches++
		m.lastRenderMS = renderMS
	}

	m.serverRate.observe(serverFrames)
	m.renderRate.observe(m.renderedBatches)
}

func (m *streamMetrics) recordDecodeDrop() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decodeDrops++
}

func (m *streamMetrics) recordEncodeFailure() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.encodeFailures++
}

func (m *streamMetrics) recordSlowClient() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.slowClient++
}

func (m *streamMetrics) addClient(delta int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients += delta
	if m.clients < 0 {
		m.clients = 0
	}
}

func (m *streamMetrics) observeServerFrames(value uint64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serverRate.observe(value)
	m.renderRate.observe(m.renderedBatches)
}

func (m *streamMetrics) snapshot(serverFrames uint64) metricsSnapshot {
	if m == nil {
		return metricsSnapshot{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serverRate.observe(serverFrames)
	return metricsSnapshot{
		batchesSent:     m.batchesSent,
		keyFramesSent:   m.keyFramesSent,
		bytesSent:       m.bytesSent,
		rectsSent:       m.rectsSent,
		decodeDrops:     m.decodeDrops,
		encodeFailures:  m.encodeFailures,
		lastSnapshotMS:  m.lastSnapshotMS,
		lastEncodeMS:    m.lastEncodeMS,
		lastSendMS:      m.lastSendMS,
		lastBatchRects:  m.lastBatchRects,
		lastBatchBytes:  m.lastBatchBytes,
		inFlight:        m.inFlight,
		inFlightBytes:   m.inFlightBytes,
		clients:         m.clients,
		slowClient:      m.slowClient,
		renderedBatches: m.renderedBatches,
		lastRenderMS:    m.lastRenderMS,
		serverFPS:       m.serverRate.rate(),
		renderFPS:       m.renderRate.rate(),
	}
}
