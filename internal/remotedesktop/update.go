package remotedesktop

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"sync"
)

const snapshotPoolMaxBufferBytes = 8 << 20

const snapshotPoolMaxFreeBuffers = 8

type snapshotPool struct {
	mu   sync.Mutex
	free [][]byte
}

func newSnapshotPool() *snapshotPool { return &snapshotPool{} }

func (p *snapshotPool) get(size int) []byte {
	if size <= 0 {
		return nil
	}
	p.mu.Lock()
	for index := len(p.free) - 1; index >= 0; index-- {
		buffer := p.free[index]
		if cap(buffer) < size {
			continue
		}
		p.free = append(p.free[:index], p.free[index+1:]...)
		p.mu.Unlock()
		return buffer[:size]
	}
	p.mu.Unlock()
	return make([]byte, size)
}

func (p *snapshotPool) put(buffer []byte) {
	if buffer == nil || cap(buffer) > snapshotPoolMaxBufferBytes {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.free) >= snapshotPoolMaxFreeBuffers {
		return
	}
	p.free = append(p.free, buffer[:cap(buffer)])
}

type RectSnapshot struct {
	Rect
	Pixels []byte
}

type CanvasUpdate struct {
	Version uint64
	Width   int
	Height  int

	KeyFrame bool
	Rects    []RectSnapshot

	pool *snapshotPool
}

func (u *CanvasUpdate) Empty() bool { return u == nil || len(u.Rects) == 0 }

func (u *CanvasUpdate) PixelBytes() int {
	if u == nil {
		return 0
	}
	total := 0
	for _, rect := range u.Rects {
		total += len(rect.Pixels)
	}
	return total
}

func (u *CanvasUpdate) Release() {
	if u == nil || u.pool == nil {
		return
	}
	for index := range u.Rects {
		u.pool.put(u.Rects[index].Pixels)
		u.Rects[index].Pixels = nil
	}
	u.Rects = nil
	u.pool = nil
}

type updateFormat uint8

const (
	updateFormatRGBA updateFormat = 1

	updateFormatPNG updateFormat = 2
)

const (
	rgbaDirectLimitBytes = 512 << 10

	pngCandidateLimitBytes = 4 << 20

	pngWorthwhileRatio = 0.75

	rgbaBandBytes = 256 << 10
)

type EncodedRect struct {
	Rect
	Format  updateFormat
	Payload []byte
}

func encodeCanvasUpdate(update *CanvasUpdate) ([]EncodedRect, error) {
	if update.Empty() {
		return nil, nil
	}
	encoded := make([]EncodedRect, 0, len(update.Rects))
	for _, rect := range update.Rects {
		items, err := encodeRectSnapshot(rect)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, items...)
	}
	return encoded, nil
}

func encodeRectSnapshot(snapshot RectSnapshot) ([]EncodedRect, error) {
	raw := len(snapshot.Pixels)
	if raw == 0 || snapshot.Width <= 0 || snapshot.Height <= 0 {
		return nil, nil
	}
	if raw <= rgbaDirectLimitBytes {
		return []EncodedRect{{Rect: snapshot.Rect, Format: updateFormatRGBA, Payload: snapshot.Pixels}}, nil
	}
	if raw <= pngCandidateLimitBytes && mayCompress(snapshot.Pixels) {
		payload, err := encodePNGBestSpeed(snapshot)
		if err == nil && len(payload) > 0 && float64(len(payload)) <= float64(raw)*pngWorthwhileRatio {
			return []EncodedRect{{Rect: snapshot.Rect, Format: updateFormatPNG, Payload: payload}}, nil
		}

	}
	return splitRGBABands(snapshot), nil
}

func mayCompress(pixels []byte) bool {
	const sampleBytes = 4096
	const distinctLimit = 192
	if len(pixels) <= sampleBytes {
		return true
	}
	step := len(pixels) / sampleBytes
	if step < 1 {
		step = 1
	}
	var seen [256]bool
	distinct := 0
	for index := 0; index < len(pixels); index += step {
		value := pixels[index]
		if seen[value] {
			continue
		}
		seen[value] = true
		distinct++
		if distinct > distinctLimit {
			return false
		}
	}
	return true
}

func encodePNGBestSpeed(snapshot RectSnapshot) ([]byte, error) {
	imageValue := image.NewRGBA(image.Rect(0, 0, snapshot.Width, snapshot.Height))
	if len(snapshot.Pixels) < len(imageValue.Pix) {
		return nil, fmt.Errorf("像素快照不足: 需要 %d 字节，实际 %d", len(imageValue.Pix), len(snapshot.Pixels))
	}
	copy(imageValue.Pix, snapshot.Pixels)
	var buffer bytes.Buffer
	buffer.Grow(len(snapshot.Pixels) / 8)
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&buffer, imageValue); err != nil {
		return nil, fmt.Errorf("编码画面失败: %w", err)
	}
	return buffer.Bytes(), nil
}

func splitRGBABands(snapshot RectSnapshot) []EncodedRect {
	rowBytes := snapshot.Width * 4
	if rowBytes <= 0 || snapshot.Height <= 0 {
		return nil
	}
	rowsPerBand := max(rgbaBandBytes/rowBytes, 1)
	bands := make([]EncodedRect, 0, (snapshot.Height+rowsPerBand-1)/rowsPerBand)
	for top := 0; top < snapshot.Height; top += rowsPerBand {
		height := min(rowsPerBand, snapshot.Height-top)
		bands = append(bands, EncodedRect{
			Rect: Rect{
				X:      snapshot.X,
				Y:      snapshot.Y + top,
				Width:  snapshot.Width,
				Height: height,
			},
			Format:  updateFormatRGBA,
			Payload: snapshot.Pixels[top*rowBytes : (top+height)*rowBytes],
		})
	}
	return bands
}
