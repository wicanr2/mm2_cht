package mobileui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/ui"
)

var (
	background = color.RGBA{19, 25, 35, 255}
	gold       = color.RGBA{233, 205, 150, 255}
	ink        = color.RGBA{233, 215, 183, 255}
	muted      = color.RGBA{157, 170, 166, 255}
)

func box(x, y, w, h int) image.Rectangle {
	return image.Rect(x*Scale, y*Scale, (x+w)*Scale, (y+h)*Scale)
}
func scaled(r image.Rectangle) image.Rectangle {
	return image.Rect(r.Min.X*Scale, r.Min.Y*Scale, r.Max.X*Scale, r.Max.Y*Scale)
}
func fill(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(dst, r, &image.Uniform{c}, image.Point{}, draw.Src)
}

func stretch(dst *image.RGBA, r image.Rectangle, src image.Image, sr image.Rectangle) {
	if r.Empty() || sr.Empty() {
		return
	}
	at := func(x, y int) color.RGBA { return color.RGBAModel.Convert(src.At(x, y)).(color.RGBA) }
	if rgba, ok := src.(*image.RGBA); ok {
		at = rgba.RGBAAt
	}
	// Bilinear interpolation on premultiplied colors, using only the standard
	// library so Android and desktop retain the existing dependency inventory.
	for y := max(0, r.Min.Y); y < min(dst.Rect.Max.Y, r.Max.Y); y++ {
		fy := (float64(y-r.Min.Y)+.5)*float64(sr.Dy())/float64(r.Dy()) - .5
		iy := int(math.Floor(fy))
		wy := fy - float64(iy)
		ya := max(sr.Min.Y, min(sr.Max.Y-1, sr.Min.Y+iy))
		yb := max(sr.Min.Y, min(sr.Max.Y-1, ya+1))
		for x := max(0, r.Min.X); x < min(dst.Rect.Max.X, r.Max.X); x++ {
			fx := (float64(x-r.Min.X)+.5)*float64(sr.Dx())/float64(r.Dx()) - .5
			ix := int(math.Floor(fx))
			wx := fx - float64(ix)
			xa := max(sr.Min.X, min(sr.Max.X-1, sr.Min.X+ix))
			xb := max(sr.Min.X, min(sr.Max.X-1, xa+1))
			a := at(xa, ya)
			b := at(xb, ya)
			cc := at(xa, yb)
			d := at(xb, yb)
			mix := func(a, b, c, d uint8) uint32 {
				return uint32((float64(a)*(1-wx)+float64(b)*wx)*(1-wy) + (float64(c)*(1-wx)+float64(d)*wx)*wy)
			}
			alpha := mix(a.A, b.A, cc.A, d.A)
			off := dst.PixOffset(x, y)
			for channel, values := range [][4]uint8{{a.R, b.R, cc.R, d.R}, {a.G, b.G, cc.G, d.G}, {a.B, b.B, cc.B, d.B}} {
				v := mix(values[0], values[1], values[2], values[3])
				dst.Pix[off+channel] = uint8(min(255, v+uint32(dst.Pix[off+channel])*(255-alpha)/255))
			}
			dst.Pix[off+3] = 255
		}
	}
}

func fit(dst *image.RGBA, r image.Rectangle, src image.Image, sr image.Rectangle) {
	f := math.Min(float64(r.Dx())/float64(sr.Dx()), float64(r.Dy())/float64(sr.Dy()))
	w, h := int(float64(sr.Dx())*f), int(float64(sr.Dy())*f)
	x, y := r.Min.X+(r.Dx()-w)/2, r.Min.Y+(r.Dy()-h)/2
	stretch(dst, image.Rect(x, y, x+w, y+h), src, sr)
}

func (c *Controller) frame(dst *image.RGBA, r image.Rectangle, border int) {
	fill(dst, r, color.RGBA{23, 29, 38, 255})
	if c.Frame == nil {
		return
	}
	s := c.Frame.Bounds()
	b := border * Scale
	sx := []int{s.Min.X, s.Min.X + 34, s.Max.X - 34, s.Max.X}
	sy := []int{s.Min.Y, s.Min.Y + 34, s.Max.Y - 34, s.Max.Y}
	dx := []int{r.Min.X, r.Min.X + b, r.Max.X - b, r.Max.X}
	dy := []int{r.Min.Y, r.Min.Y + b, r.Max.Y - b, r.Max.Y}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if x == 1 && y == 1 {
				continue
			}
			sr := image.Rect(sx[x], sy[y], sx[x+1], sy[y+1])
			dr := image.Rect(dx[x], dy[y], dx[x+1], dy[y+1])
			nx, ny := 1, 1
			if x == 1 {
				natural := float64(sr.Dx()) * float64(b) / 34
				nx = max(1, int(math.Round(float64(dr.Dx())/natural)))
			}
			if y == 1 {
				natural := float64(sr.Dy()) * float64(b) / 34
				ny = max(1, int(math.Round(float64(dr.Dy())/natural)))
			}
			for j := 0; j < ny; j++ {
				for i := 0; i < nx; i++ {
					tile := image.Rect(dr.Min.X+i*dr.Dx()/nx, dr.Min.Y+j*dr.Dy()/ny, dr.Min.X+(i+1)*dr.Dx()/nx, dr.Min.Y+(j+1)*dr.Dy()/ny)
					stretch(dst, tile, c.Frame, sr)
				}
			}
		}
	}
}

// Text is bounded in both axes; truncation is explicit and uses the same font
// advances as painting. Font coverage retains antialiasing at phone density.
func (c *Controller) text(dst *image.RGBA, text string, r image.Rectangle, size int, col color.RGBA, center bool) {
	text = c.Convert(strings.ReplaceAll(text, "@", " "))
	d := c.Session.DisplayCatalog()
	cf, lf := d.Font(c.Session.Presentation().Locale), d.Sans
	advance := func(ch rune) int {
		if ch < 128 {
			return size * Scale / 2
		}
		return size * Scale
	}
	width := func(s string) int {
		n := 0
		for _, ch := range s {
			n += advance(ch)
		}
		return n
	}
	chars := []rune(text)
	for len(chars) > 0 && width(string(chars)) > r.Dx() {
		chars = chars[:len(chars)-1]
	}
	if len(chars) < len([]rune(text)) && len(chars) > 0 {
		for len(chars) > 0 && width(string(chars)+"…") > r.Dx() {
			chars = chars[:len(chars)-1]
		}
		if width("…") <= r.Dx() {
			chars = append(chars, '…')
		}
	}
	w := width(string(chars))
	x := r.Min.X
	if center {
		x += (r.Dx() - w) / 2
	}
	h := size * Scale
	y := r.Min.Y + (r.Dy()-h)/2
	if h > r.Dy() {
		return
	}
	for _, ch := range chars {
		f := cf
		if ch < 128 {
			f = lf
		}
		gw := advance(ch)
		for gy := 0; gy < h; gy++ {
			for gx := 0; gx < gw; gx++ {
				px, py := x+gx, y+gy
				if !image.Pt(px, py).In(r) || !image.Pt(px, py).In(dst.Bounds()) {
					continue
				}
				a := uint32(f.Coverage(ch, gx*f.W/gw, gy*f.H/h))
				if a == 0 {
					continue
				}
				bg := dst.RGBAAt(px, py)
				dst.SetRGBA(px, py, color.RGBA{uint8((uint32(col.R)*a + uint32(bg.R)*(255-a)) / 255), uint8((uint32(col.G)*a + uint32(bg.G)*(255-a)) / 255), uint8((uint32(col.B)*a + uint32(bg.B)*(255-a)) / 255), 255})
			}
		}
		x += gw
	}
}

func (c *Controller) line(dst *image.RGBA, x1, y1, x2, y2 int, col color.RGBA) {
	dx, dy := x2-x1, y2-y1
	n := max(abs(dx), abs(dy))
	if n == 0 {
		n = 1
	}
	for i := 0; i <= n; i++ {
		x, y := (x1+dx*i/n)*Scale, (y1+dy*i/n)*Scale
		fill(dst, image.Rect(x, y, x+Scale, y+Scale), col)
	}
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func gem(dst *image.RGBA, x, y, w, h int) {
	for py := 0; py < h*Scale; py++ {
		distance := math.Abs((float64(py)+.5)/float64(h*Scale)*2 - 1)
		half := float64(w*Scale) / 2 * (1 - distance)
		for px := 0; px < w*Scale; px++ {
			edge := math.Abs(float64(px) + .5 - float64(w*Scale)/2)
			if edge > half {
				continue
			}
			col := color.RGBA{140, 22, 48, 255}
			if edge > half-float64(Scale) {
				col = gold
			} else if px < w*Scale/2 && py < h*Scale/2 {
				col = color.RGBA{227, 85, 88, 255}
			}
			dst.SetRGBA(x*Scale+px, y*Scale+py, col)
		}
	}
}
func (c *Controller) icon(dst *image.RGBA, name string, x, y int, col color.RGBA) {
	line := func(a, b, d, e int) { c.line(dst, x+a, y+b, x+d, y+e, col) }
	switch name {
	case "compass":
		line(11, 2, 11, 20)
		line(2, 11, 20, 11)
		line(5, 5, 17, 17)
		line(5, 17, 17, 5)
		line(8, 8, 14, 8)
		line(14, 8, 14, 14)
		line(14, 14, 8, 14)
		line(8, 14, 8, 8)
	case "up":
		line(11, 2, 11, 21)
		line(4, 9, 11, 2)
		line(11, 2, 18, 9)
	case "down":
		line(11, 2, 11, 21)
		line(4, 14, 11, 21)
		line(11, 21, 18, 14)
	case "left":
		line(10, 3, 3, 9)
		line(3, 9, 10, 15)
		line(3, 9, 16, 9)
		line(16, 9, 20, 13)
		line(20, 13, 20, 21)
	case "right":
		line(13, 3, 20, 9)
		line(20, 9, 13, 15)
		line(20, 9, 7, 9)
		line(7, 9, 3, 13)
		line(3, 13, 3, 21)
	case "more":
		for _, b := range []int{5, 11, 17} {
			line(3, b, 20, b)
		}
	case "check":
		line(3, 11, 9, 18)
		line(9, 18, 21, 4)
	case "back":
		line(9, 4, 2, 11)
		line(2, 11, 9, 18)
		line(2, 11, 18, 11)
		line(18, 11, 21, 16)
		line(21, 16, 18, 21)
	case "search":
		for a := 0.; a < 6.3; a += .08 {
			xx, yy := x+9+int(6*math.Cos(a)), y+9+int(6*math.Sin(a))
			fill(dst, box(xx, yy, 1, 1), col)
		}
		line(14, 14, 21, 21)
	case "map":
		line(2, 5, 8, 3)
		line(8, 3, 15, 5)
		line(15, 5, 21, 3)
		line(2, 5, 2, 21)
		line(2, 21, 8, 19)
		line(8, 19, 15, 21)
		line(15, 21, 21, 19)
		line(21, 19, 21, 3)
		line(8, 3, 8, 19)
		line(15, 5, 15, 21)
	case "items":
		line(7, 6, 7, 3)
		line(7, 3, 16, 3)
		line(16, 3, 16, 6)
		line(5, 7, 18, 7)
		line(5, 7, 3, 21)
		line(18, 7, 20, 21)
		line(3, 21, 20, 21)
		line(4, 12, 19, 12)
		line(10, 10, 10, 15)
		line(10, 15, 14, 15)
		line(14, 15, 14, 10)
	case "sword", "magic":
		line(3, 21, 20, 3)
		line(6, 11, 13, 18)
		if name == "magic" {
			line(17, 1, 17, 8)
			line(14, 4, 21, 4)
		} else {
			line(17, 2, 22, 1)
			line(22, 1, 21, 6)
		}
	default:
		line(11, 2, 20, 5)
		line(20, 5, 19, 15)
		line(19, 15, 11, 22)
		line(11, 22, 4, 15)
		line(4, 15, 3, 5)
		line(3, 5, 11, 2)
		line(11, 6, 11, 17)
		line(8, 9, 14, 9)
	}
}

func (c *Controller) Draw() *image.RGBA {
	s := c.Session
	dst := image.NewRGBA(box(0, 0, Width, Height))
	fill(dst, dst.Bounds(), background)
	c.text(dst, c.Text("title"), box(42, 10, 280, 40), 17, gold, false)
	gem(dst, 22, 19, 12, 17)
	fill(dst, box(22, 48, 800, 1), color.RGBA{136, 113, 69, 255})
	c.text(dst, s.Mode.String(), box(20, 67, 145, 26), 13, gold, true)
	c.icon(dst, "compass", 88, 197, color.RGBA{165, 139, 95, 255})
	directions := []string{"north", "east", "south", "west"}
	c.text(dst, s.LocationLabel()+" · "+c.Text(directions[int(s.Game.World.Face)%4]), box(570, 11, 250, 37), 12, muted, false)
	c.frame(dst, box(188, 54, 468, 258), 9)
	gameFrame := s.Draw().Hi
	opaque := c.isTextPage() || c.Overlay != "" || s.Mode == ui.ModeMenu || s.Mode == ui.ModeCreate || s.Mode == ui.ModeName
	if !opaque && (s.Mode == ui.ModeIntro || s.Mode == ui.ModeMap || s.Mode == ui.ModeWorld) {
		sr := gameFrame.Bounds()
		if c.MapZoom && (s.Mode == ui.ModeMap || s.Mode == ui.ModeWorld) {
			sr = image.Rect(0, 0, 640, 400)
		}
		fit(dst, box(197, 63, 450, 240), gameFrame, sr)
	} else if !opaque {
		fit(dst, box(197, 63, 450, 240), gameFrame, image.Rect(44, 40, 652, 375))
	}
	title, opts := c.choices()
	if c.Overlay != "" || s.Mode == ui.ModeMenu {
		fill(dst, box(197, 63, 450, 240), color.RGBA{18, 27, 36, 255})
		c.text(dst, title, box(208, 65, 428, 30), 18, gold, false)
		if len(opts) > 3 && c.Overlay != "classes" {
			c.text(dst, fmt.Sprintf("%d/%d", c.Page+1, (len(opts)+2)/3), box(574, 70, 62, 22), 11, muted, true)
		}
	}
	if s.Mode == ui.ModeCreate && c.Overlay == "" {
		fill(dst, box(197, 63, 450, 240), color.RGBA{18, 27, 36, 255})
		for i, line := range s.CreateLines() {
			if i >= 16 {
				break
			}
			c.text(dst, line, box(208, 70+i*14, 428, 14), 12, ink, false)
		}
	}
	if _, active := s.InputValue(); active && c.Overlay == "" && s.Mode == ui.ModeName {
		fill(dst, box(197, 63, 450, 240), color.RGBA{18, 27, 36, 255})
		lines := s.NameLines()
		if len(lines) > 0 {
			lines[0] = c.Text("name_title")
		}
		for i, line := range lines {
			if i >= 10 {
				break
			}
			c.text(dst, line, box(208, 78+i*20, 428, 20), 14, ink, false)
		}
	}
	if c.isTextPage() {
		fill(dst, box(197, 63, 450, 240), color.RGBA{18, 27, 36, 255})
		title, lines := c.textContent()
		c.text(dst, title, box(208, 65, 428, 30), 18, gold, false)
		page := max(0, min(c.Page, c.textPages()-1))
		start := page * 10
		for i := start; i < len(lines) && i < start+10; i++ {
			c.text(dst, lines[i], box(208, 98+(i-start)*18, 428, 18), 14, ink, false)
		}
		if c.textPages() > 1 {
			c.text(dst, fmt.Sprintf("%d / %d", page+1, c.textPages()), box(530, 280, 106, 20), 11, muted, true)
		}
	}
	for _, t := range c.Targets() {
		if strings.HasPrefix(t.ID, "party:") {
			var i int
			fmt.Sscanf(t.ID, "party:%d", &i)
			p := s.Game.Party[i]
			r := scaled(t.Box)
			border := color.RGBA{134, 112, 71, 255}
			if i == c.Selected {
				border = gold
			}
			fill(dst, r, border)
			fill(dst, r.Inset(Scale), color.RGBA{31, 35, 43, 255})
			c.text(dst, fmt.Sprintf("%d %s", i+1, p.Name), r.Inset(4*Scale).Intersect(box(t.Box.Min.X+4, t.Box.Min.Y+2, 66, 22)), 13, ink, false)
			c.text(dst, fmt.Sprintf("%s %d", p.Class, p.HP), box(t.Box.Min.X+4, 341, 66, 20), 12, muted, false)
			w := 0
			if p.MaxHP > 0 {
				w = max(0, min(70, int(p.HP)*70/int(p.MaxHP)))
			}
			fill(dst, box(t.Box.Min.X+2, 365, w, 3), color.RGBA{99, 141, 112, 255})
			continue
		}
		if t.ID == "" {
			c.text(dst, t.Label, scaled(t.Box).Inset(3*Scale), 13, ink, false)
			continue
		}
		c.frame(dst, scaled(t.Box), 8)
		gem(dst, (t.Box.Min.X+t.Box.Max.X)/2-3, t.Box.Min.Y+1, 6, 8)
		col := ink
		if !t.Enabled {
			col = color.RGBA{97, 102, 109, 255}
		}
		if c.pressed == t.ID && !c.cancelled {
			fill(dst, scaled(t.Box).Inset(8*Scale), color.RGBA{75, 57, 36, 255})
		}
		if s.Mode == ui.ModeMenu && s.Menu != nil && t.ID == fmt.Sprintf("menu:%d", s.Menu.Cur) {
			fill(dst, scaled(t.Box).Inset(8*Scale), color.RGBA{65, 49, 32, 255})
			col = gold
		}
		if t.Icon != "" {
			cx := (t.Box.Min.X + t.Box.Max.X) / 2
			c.icon(dst, t.Icon, cx-11, t.Box.Min.Y+9, col)
			c.text(dst, t.Label, box(t.Box.Min.X+6, t.Box.Min.Y+31, t.Box.Dx()-12, t.Box.Dy()-36), 13, col, true)
		} else {
			c.text(dst, t.Label, scaled(t.Box).Inset(10*Scale), 14, col, false)
		}
		if !t.Enabled {
			r := scaled(t.Box).Intersect(dst.Bounds())
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					off := dst.PixOffset(x, y)
					for i, bg := range []uint8{background.R, background.G, background.B} {
						dst.Pix[off+i] = uint8((uint32(dst.Pix[off+i])*35 + uint32(bg)*65) / 100)
					}
				}
			}
		}
	}
	c.text(dst, c.Text("step_hint"), box(20, 323, 142, 22), 11, muted, true)
	c.text(dst, c.Text("stop_hint"), box(20, 343, 142, 20), 11, muted, true)
	message := s.Message()
	if s.ItemsOpen() {
		message = c.Text("item_hint")
	} else if s.Mode == ui.ModeMenu {
		message = c.Text("menu_hint")
	}
	if c.Status != "" {
		message = c.Status
	}
	if message == "" {
		message = c.Text("ready")
	}
	c.text(dst, message, box(188, 371, 468, 17), 10, muted, false)
	return dst
}
