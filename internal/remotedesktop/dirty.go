package remotedesktop

type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func (r Rect) empty() bool { return r.Width <= 0 || r.Height <= 0 }

func (r Rect) area() int { return r.Width * r.Height }

func (r Rect) right() int  { return r.X + r.Width }
func (r Rect) bottom() int { return r.Y + r.Height }

func (r Rect) union(other Rect) Rect {
	left := min(r.X, other.X)
	top := min(r.Y, other.Y)
	right := max(r.right(), other.right())
	bottom := max(r.bottom(), other.bottom())
	return Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

func (r Rect) clamp(width, height int) Rect {
	left := max(r.X, 0)
	top := max(r.Y, 0)
	right := min(r.right(), width)
	bottom := min(r.bottom(), height)
	return Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

type dirtyLimits struct {
	maxRects int

	maxAreaRatio float64

	mergeGap int
}

func defaultDirtyLimits() dirtyLimits {
	return dirtyLimits{maxRects: 16, maxAreaRatio: 0.6, mergeGap: 24}
}

type dirtyRegion struct {
	limits        dirtyLimits
	width, height int
	rects         []Rect

	full bool
}

func newDirtyRegion(width, height int) dirtyRegion {
	return dirtyRegion{
		limits: defaultDirtyLimits(),
		width:  width,
		height: height,
		full:   true,
	}
}

func (d *dirtyRegion) resize(width, height int) {
	d.width = width
	d.height = height
	d.rects = nil
	d.full = true
}

func (d *dirtyRegion) mark(rect Rect) {
	if d.full {

		return
	}
	rect = rect.clamp(d.width, d.height)
	if rect.empty() {
		return
	}
	if d.areaRatio(rect.area()) >= d.limits.maxAreaRatio {
		d.collapseToKeyFrame()
		return
	}
	d.rects = append(d.rects, rect)
	d.mergeRectangles()
	if len(d.rects) > d.limits.maxRects {
		bounding := d.boundingBox()
		if bounding.empty() || d.areaRatio(bounding.area()) >= d.limits.maxAreaRatio {
			d.collapseToKeyFrame()
			return
		}
		d.rects = []Rect{bounding}
	}
}

func (d *dirtyRegion) take() (rects []Rect, full bool) {
	rects = d.rects
	full = d.full
	d.rects = nil
	d.full = false
	return rects, full
}

func (d *dirtyRegion) pending() (count int, area int, full bool) {
	if d.full {
		return 0, d.width * d.height, true
	}
	for _, rect := range d.rects {
		area += rect.area()
	}
	return len(d.rects), area, false
}

func (d *dirtyRegion) collapseToKeyFrame() {
	d.rects = nil
	d.full = true
}

func (d *dirtyRegion) boundingBox() Rect {
	if len(d.rects) == 0 {
		return Rect{}
	}
	box := d.rects[0]
	for _, rect := range d.rects[1:] {
		box = box.union(rect)
	}
	return box
}

func (d *dirtyRegion) areaRatio(area int) float64 {
	total := d.width * d.height
	if total <= 0 {
		return 1
	}
	return float64(area) / float64(total)
}

func (d *dirtyRegion) mergeRectangles() {
	for {
		first, second := -1, -1
		bestWaste := 0
		for i := 0; i < len(d.rects); i++ {
			for j := i + 1; j < len(d.rects); j++ {
				union := d.rects[i].union(d.rects[j])
				waste := union.area() - d.rects[i].area() - d.rects[j].area()
				if !d.mergeWorthwhile(waste, union) {
					continue
				}
				if first < 0 || waste < bestWaste {
					first, second, bestWaste = i, j, waste
				}
			}
		}
		if first < 0 {
			return
		}
		d.rects[first] = d.rects[first].union(d.rects[second])
		d.rects = append(d.rects[:second], d.rects[second+1:]...)
	}
}

func (d *dirtyRegion) mergeWorthwhile(waste int, union Rect) bool {
	if waste <= 0 {

		return true
	}
	span := min(union.Width, union.Height)
	if span <= 0 {
		return false
	}
	return waste <= d.limits.mergeGap*span
}
