package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/wicanr2/mm2_cht/internal/i18n"
	"github.com/wicanr2/mm2_cht/internal/render"
)

// Toolbar owns presentation input, including a release after dragging outside.
type Toolbar struct {
	session     *Session
	Visible     bool
	menu        int
	down, owned bool
}

var toolbarBoxes = []image.Rectangle{
	image.Rect(12, 4, 130, 40), image.Rect(140, 4, 302, 40), image.Rect(312, 4, 540, 40),
	image.Rect(554, 4, 668, 40), image.Rect(680, 4, 780, 40), image.Rect(794, 4, 902, 40), image.Rect(914, 4, 954, 40),
}

func NewToolbar(s *Session) *Toolbar { return &Toolbar{session: s, Visible: true} }
func (t *Toolbar) RowHeight() int    { return 40 + (t.session.Presentation().ToolbarPercent-100)*16/50 }

// Frame keeps the approved border and all six party rows below the controls.
func (t *Toolbar) Frame(s *render.Screen) *image.RGBA {
	h := t.RowHeight()
	out := image.NewRGBA(image.Rect(0, 0, render.HiW, render.HiH+h))
	fillRGBA(out, image.Rect(0, 0, render.HiW, h), color.RGBA{17, 31, 48, 255})
	draw.Draw(out, image.Rect(0, h, render.HiW, render.HiH+h), s.Hi, image.Point{}, draw.Src)
	t.Draw(&render.Screen{Hi: out})
	return out
}
func (t *Toolbar) box(index, row int) image.Rectangle {
	b := toolbarBoxes[index]
	h := t.RowHeight()
	return image.Rect(b.Min.X, row*h+4, b.Max.X, (row+1)*h-4)
}

func (t *Toolbar) choices() []string {
	p, d := t.session.Presentation(), t.session.DisplayCatalog()
	switch t.menu {
	case 1:
		return []string{d.Text(p.Locale, "toolbar.traditional"), d.Text(p.Locale, "toolbar.simplified")}
	case 2:
		return []string{"80%", "90%", "100%"}
	case 3:
		var out []string
		for _, id := range t.session.Platforms() {
			out = append(out, t.platformLabel(id))
		}
		return out
	}
	return nil
}
func (t *Toolbar) platformLabel(id string) string {
	switch id {
	case "dos":
		return "DOS"
	case "amiga":
		return "Amiga"
	case "msx":
		return "MSX"
	case "megadrive":
		return "Mega Drive"
	default:
		return t.session.DisplayCatalog().Text(t.session.Presentation().Locale, "toolbar.modern")
	}
}

// Pointer returns whether redraw/preferences changed and whether this press is owned.
func (t *Toolbar) Pointer(x, y int, down bool) (bool, bool) {
	beforeVisible, beforeMenu := t.Visible, t.menu
	changed := false
	inside := x >= 0 && x < render.HiW && y >= 0
	if inside && y < 8 {
		t.Visible = true
	}
	if t.menu == 0 && (!inside || y >= t.RowHeight()) {
		t.Visible = false
	}
	press := down && !t.down
	if press {
		t.owned = t.Visible || t.menu != 0
		if t.Visible {
			p := t.session.Presentation()
			selected := -1
			for i := range toolbarBoxes {
				if image.Pt(x, y).In(t.box(i, 0)) {
					selected = i
					break
				}
			}
			if selected >= 0 {
				if selected < 3 {
					if t.menu == selected+1 {
						t.menu = 0
					} else {
						t.menu = selected + 1
					}
				} else {
					t.menu = 0
					switch selected {
					case 3:
						p.HD = false
					case 4:
						if t.session.HDReady() {
							p.HD = true
						}
					case 5:
						changed = t.session.Key(KeySettings)
					case 6:
						switch p.ToolbarPercent {
						case 100:
							p.ToolbarPercent = 125
						case 125:
							p.ToolbarPercent = 150
						default:
							p.ToolbarPercent = 100
						}
					}
					if t.session.SetPresentation(p) == nil {
						changed = true
					}
				}
			} else if t.menu != 0 {
				menu := t.menu
				choices := t.choices()
				t.menu = 0
				for i := range choices {
					if image.Pt(x, y).In(t.box(menu-1, i+1)) {
						switch menu {
						case 1:
							p.Locale = []string{i18n.Traditional, i18n.Simplified}[i]
						case 2:
							p.TextPercent = []int{80, 90, 100}[i]
						case 3:
							p.Platform = t.session.Platforms()[i]
							if p.Platform != "amiga" {
								p.HD = false
							}
						}
						if t.session.SetPresentation(p) == nil {
							changed = true
						}
						break
					}
				}
			}
		}
	}
	owned := t.owned
	if !down {
		t.owned = false
	}
	t.down = down
	return changed || beforeVisible != t.Visible || beforeMenu != t.menu, owned
}

func (t *Toolbar) CloseMenu() bool {
	if t.menu == 0 {
		return false
	}
	t.menu = 0
	return true
}
func (t *Toolbar) MenuOpen() bool { return t.menu != 0 }

func (t *Toolbar) Draw(s *render.Screen) {
	if !t.Visible {
		return
	}
	p, d := t.session.Presentation(), t.session.DisplayCatalog()
	h := t.RowHeight()
	navy := color.RGBA{17, 31, 48, 255}
	fillRGBA(s.Hi, image.Rect(0, 0, render.HiW, h), navy)
	labels := []string{d.Text(p.Locale, "toolbar.language"), fmt.Sprintf("%s %d%%", d.Text(p.Locale, "toolbar.text"), p.TextPercent), d.Text(p.Locale, "toolbar.assets") + " " + t.platformLabel(p.Platform), d.Text(p.Locale, "toolbar.original"), d.Text(p.Locale, "toolbar.hd"), d.Text(p.Locale, "toolbar.settings"), "A+"}
	if !t.session.HDReady() {
		labels[4] = d.Text(p.Locale, "toolbar.unavailable")
	}
	for i, label := range labels {
		active := (i == 3 && !p.HD) || (i == 4 && p.HD) || (i < 3 && t.menu == i+1)
		enabled := i != 4 || t.session.HDReady()
		t.drawButton(s.Hi, t.box(i, 0), label, active, enabled, i < 3)
		if i < 3 {
			b := t.box(i, 0)
			cy := (b.Min.Y+b.Max.Y)/2 - 2
			for row := 0; row < 4; row++ {
				for dx := -3 + row; dx <= 3-row; dx++ {
					s.Hi.SetRGBA(b.Max.X-10+dx, cy+row, color.RGBA{225, 205, 160, 255})
				}
			}
		}
	}
	if t.menu != 0 {
		for i, label := range t.choices() {
			box := t.box(t.menu-1, i+1)
			fillRGBA(s.Hi, image.Rect(box.Min.X-4, box.Min.Y-4, box.Max.X+4, box.Max.Y+4), navy)
			active := t.menu == 1 && (i == 0 && p.Locale == i18n.Traditional || i == 1 && p.Locale == i18n.Simplified) || t.menu == 2 && []int{80, 90, 100}[i] == p.TextPercent
			if t.menu == 3 {
				active = t.session.Platforms()[i] == p.Platform
			}
			t.drawButton(s.Hi, box, label, active, true, false)
		}
	}
}

func fillRGBA(dst *image.RGBA, box image.Rectangle, c color.RGBA) {
	draw.Draw(dst, box, &image.Uniform{c}, image.Point{}, draw.Src)
}

func (t *Toolbar) drawButton(dst *image.RGBA, box image.Rectangle, label string, active, enabled, caret bool) {
	border := color.RGBA{133, 148, 163, 255}
	ink := color.RGBA{207, 217, 228, 255}
	if active {
		border = color.RGBA{225, 190, 100, 255}
		ink = color.RGBA{249, 217, 148, 255}
	}
	if !enabled {
		border = color.RGBA{64, 77, 91, 255}
		ink = color.RGBA{93, 107, 121, 255}
	}
	fillRGBA(dst, box, border)
	fillRGBA(dst, box.Inset(1), color.RGBA{28, 43, 60, 255})
	d := t.session.DisplayCatalog()
	p := t.session.Presentation()
	cjkFont, latinFont := d.ToolbarFonts(p.Locale, p.ToolbarPercent)
	st := render.TextStyle{CJK: cjkFont, Latin: latinFont, Color: ink}
	w := 0
	for _, r := range label {
		w += st.Advance(r)
	}
	if w == 0 {
		return
	}
	nativeHeight := cjkFont.H
	temp := image.NewRGBA(image.Rect(0, 0, w, nativeHeight))
	scr := &render.Screen{Hi: temp}
	scr.DrawText(st, label, 0, 0)
	// Long platform names may use less horizontal space, but must retain the
	// same line height as the other buttons and the dropdown choices.
	padding := 12
	if caret {
		padding = 28
	}
	tw, th := max(1, min(w, box.Dx()-padding)), max(1, min(nativeHeight, box.Dy()-6))
	ox, oy := (box.Min.X+box.Max.X-tw)/2, (box.Min.Y+box.Max.Y-th)/2
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			c := temp.RGBAAt(x*w/tw, y*nativeHeight/th)
			a := max(c.R, c.G, c.B)
			if a == 0 {
				continue
			}
			bg := dst.RGBAAt(ox+x, oy+y)
			dst.SetRGBA(ox+x, oy+y, color.RGBA{uint8(min(255, int(c.R)+int(bg.R)*(255-int(a))/255)), uint8(min(255, int(c.G)+int(bg.G)*(255-int(a))/255)), uint8(min(255, int(c.B)+int(bg.B)*(255-int(a))/255)), 255})
		}
	}
}
