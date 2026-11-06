package remotedesktop

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"
)

type Framebuffer struct {
	Width  int
	Height int

	mu      sync.RWMutex
	pixels  []byte
	version uint64

	dirty dirtyRegion

	snapshots *snapshotPool

	updates uint64

	activity chan struct{}
}

func NewFramebuffer(width, height int) *Framebuffer {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	buffer := &Framebuffer{
		Width:     width,
		Height:    height,
		pixels:    make([]byte, width*height*4),
		dirty:     newDirtyRegion(width, height),
		snapshots: newSnapshotPool(),
		activity:  make(chan struct{}, 1),
	}
	for index := 3; index < len(buffer.pixels); index += 4 {
		buffer.pixels[index] = 0xFF
	}
	return buffer
}

func (f *Framebuffer) Size() (width, height int) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Width, f.Height
}

func (f *Framebuffer) Resize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Width == width && f.Height == height {
		return
	}
	f.Width = width
	f.Height = height
	f.pixels = make([]byte, width*height*4)
	for index := 3; index < len(f.pixels); index += 4 {
		f.pixels[index] = 0xFF
	}
	f.version++
	f.updates++

	f.dirty.resize(width, height)
	f.signalActivity()
}

func (f *Framebuffer) Activity() <-chan struct{} { return f.activity }

func (f *Framebuffer) signalActivity() {
	select {
	case f.activity <- struct{}{}:
	default:
	}
}

func (f *Framebuffer) Pixels() []byte {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]byte(nil), f.pixels...)
}

func (f *Framebuffer) Version() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.version
}

func (f *Framebuffer) BlitRGBA(x, y, width, height, stride int, pixels []byte) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("RGBA 贴图尺寸非法: %dx%d", width, height)
	}
	if stride < width*4 {
		return fmt.Errorf("RGBA 行跨度不足: stride=%d width=%d", stride, width)
	}
	if len(pixels) < stride*height {
		return fmt.Errorf("RGBA 数据不足: 需要 %d 字节，实际 %d", stride*height, len(pixels))
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	startColumn := 0
	if x < 0 {
		startColumn = -x
	}
	endColumn := width
	if x+width > f.Width {
		endColumn = f.Width - x
	}
	if startColumn >= endColumn {
		return nil
	}
	changed := false
	for row := 0; row < height; row++ {
		destinationY := y + row
		if destinationY < 0 || destinationY >= f.Height {
			continue
		}
		sourceRow := pixels[row*stride+startColumn*4 : row*stride+endColumn*4]
		destinationOffset := (destinationY*f.Width + x + startColumn) * 4
		destinationRow := f.pixels[destinationOffset : destinationOffset+len(sourceRow)]
		if bytes.Equal(destinationRow, sourceRow) && alphaOpaque(destinationRow) {

			continue
		}
		copy(destinationRow, sourceRow)

		for offset := 3; offset < len(destinationRow); offset += 4 {
			destinationRow[offset] = 0xFF
		}
		changed = true
	}
	if !changed {
		return nil
	}
	f.version++
	f.updates++

	f.dirty.mark(Rect{X: x + startColumn, Y: y, Width: endColumn - startColumn, Height: height})
	f.signalActivity()
	return nil
}

func alphaOpaque(row []byte) bool {
	for offset := 3; offset < len(row); offset += 4 {
		if row[offset] != 0xFF {
			return false
		}
	}
	return true
}

func (f *Framebuffer) FillRectangle(left, top, width, height int, red, green, blue byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	changed := false
	for y := top; y < top+height; y++ {
		if y < 0 || y >= f.Height {
			continue
		}
		for x := left; x < left+width; x++ {
			if x < 0 || x >= f.Width {
				continue
			}
			offset := (y*f.Width + x) * 4
			if f.pixels[offset] == red && f.pixels[offset+1] == green && f.pixels[offset+2] == blue && f.pixels[offset+3] == 0xFF {
				continue
			}
			f.pixels[offset] = red
			f.pixels[offset+1] = green
			f.pixels[offset+2] = blue
			f.pixels[offset+3] = 0xFF
			changed = true
		}
	}
	if !changed {
		return
	}
	f.version++
	f.updates++
	f.dirty.mark(Rect{X: left, Y: top, Width: width, Height: height})
	f.signalActivity()
}

func (f *Framebuffer) Updates() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.updates
}

func (f *Framebuffer) ForceKeyFrame() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dirty.collapseToKeyFrame()

	f.signalActivity()
}

func (f *Framebuffer) PendingDirty() (rects int, area int, full bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.dirty.pending()
}

func (f *Framebuffer) TakeUpdate() CanvasUpdate {
	f.mu.Lock()
	defer f.mu.Unlock()

	rects, full := f.dirty.take()
	width, height := f.Width, f.Height
	snapshot := CanvasUpdate{
		Version:  f.version,
		Width:    width,
		Height:   height,
		KeyFrame: full,
		pool:     f.snapshots,
	}
	if full {

		rects = []Rect{{X: 0, Y: 0, Width: width, Height: height}}
	}
	for _, rect := range rects {
		rect = rect.clamp(width, height)
		if rect.empty() {
			continue
		}
		pixels := f.snapshots.get(rect.area() * 4)
		if pixels == nil {
			continue
		}
		copyRectPixels(pixels, rect, f.pixels, width)
		snapshot.Rects = append(snapshot.Rects, RectSnapshot{Rect: rect, Pixels: pixels})
	}
	return snapshot
}

func copyRectPixels(destination []byte, rect Rect, source []byte, canvasWidth int) {
	rowBytes := rect.Width * 4
	for row := 0; row < rect.Height; row++ {
		sourceOffset := ((rect.Y+row)*canvasWidth + rect.X) * 4
		copy(destination[row*rowBytes:(row+1)*rowBytes], source[sourceOffset:sourceOffset+rowBytes])
	}
}

func (f *Framebuffer) PNG() ([]byte, error) {

	f.mu.RLock()
	width, height := f.Width, f.Height
	snapshot := append([]byte(nil), f.pixels...)
	f.mu.RUnlock()

	image := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(image.Pix, snapshot)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image); err != nil {
		return nil, fmt.Errorf("编码画面失败: %w", err)
	}
	return buffer.Bytes(), nil
}

func (f *Framebuffer) Stats() (nonBlack int, distinctColors int) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	seen := map[uint32]struct{}{}
	for offset := 0; offset+3 < len(f.pixels); offset += 4 {
		red := f.pixels[offset]
		green := f.pixels[offset+1]
		blue := f.pixels[offset+2]
		if red == 0 && green == 0 && blue == 0 {
			continue
		}
		nonBlack++
		value := uint32(red)<<16 | uint32(green)<<8 | uint32(blue)
		if len(seen) < 4096 {
			seen[value] = struct{}{}
		}
	}
	return nonBlack, len(seen)
}

func (f *Framebuffer) Checksum() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	hash := uint64(offset64)
	for _, value := range f.pixels {
		hash ^= uint64(value)
		hash *= prime64
	}
	return hash
}

func (f *Framebuffer) ColorAt(x, y int) color.RGBA {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if x < 0 || y < 0 || x >= f.Width || y >= f.Height {
		return color.RGBA{}
	}
	offset := (y*f.Width + x) * 4
	return color.RGBA{R: f.pixels[offset], G: f.pixels[offset+1], B: f.pixels[offset+2]}
}

func (f *Framebuffer) CursorFrame() (png []byte, x, y, width, height int, ok bool) {
	return nil, 0, 0, 0, 0, false
}
