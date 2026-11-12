// Package mobileui adapts the existing session to the approved landscape phone
// interface. It does not implement game rules.
package mobileui

import (
	"fmt"
	"image"
	"image/draw"
	"strconv"
	"strings"
	"time"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/i18n"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

const Width, Height = 844, 390

var Scale = 3

type Target struct {
	ID, Label, Icon string
	Box             image.Rectangle
	Enabled, Repeat bool
}

type choice struct {
	label, action string
	enabled       bool
}

type Controller struct {
	Session             *ui.Session
	Frame               image.Image
	Overlay             string
	Selected, Page      int
	Status              string
	owner               int
	pressed, stamp      string
	started, next       time.Duration
	repeated, cancelled bool
	InputToken          int
	EditorRequested     bool
	inputStamp          string
	Dirty               bool
	MapZoom             bool
	MinHit              int
	ActionSerial        int64
	LastAction          string
}

func New(s *ui.Session, frame image.Image) *Controller {
	if frame != nil {
		rgba := image.NewRGBA(frame.Bounds())
		draw.Draw(rgba, rgba.Bounds(), frame, frame.Bounds().Min, draw.Src)
		frame = rgba
	}
	return &Controller{Session: s, Frame: frame, owner: -1, Dirty: true}
}

func (c *Controller) Text(key string) string {
	s := c.Session
	return s.DisplayCatalog().Text(s.Presentation().Locale, "mobile."+key)
}
func (c *Controller) Convert(text string) string {
	return c.Session.DisplayCatalog().Convert(c.Session.Presentation().Locale, text)
}
func (c *Controller) signature() string {
	s := c.Session
	return fmt.Sprintf("%d/%p/%p/%p/%s/%d/%s", s.Mode, s.Menu, s.Game.Fight, s.Game.World.Pending, c.Overlay, c.Page, s.Notice)
}

func (c *Controller) save() {
	if c.Session.CanCheckpoint() {
		if err := c.Session.Checkpoint(); err != nil {
			c.Status = c.Text("save_failed") + err.Error()
		}
	}
}
func (c *Controller) Checkpoint()  { c.save() }
func (c *Controller) CancelTouch() { c.owner = -1; c.pressed = ""; c.cancelled = true; c.Dirty = true }
func (c *Controller) Owner() int   { return c.owner }

func (c *Controller) HitBox(t Target) image.Rectangle {
	r := t.Box
	if r.Dx() < c.MinHit {
		d := c.MinHit - r.Dx()
		r.Min.X -= d / 2
		r.Max.X += d - d/2
	}
	if r.Dy() < c.MinHit {
		d := c.MinHit - r.Dy()
		r.Min.Y -= d / 2
		r.Max.Y += d - d/2
	}
	return r
}

func (c *Controller) choices() (string, []choice) {
	s := c.Session
	switch c.Overlay {
	case "party":
		i := c.Selected
		if i >= len(s.Game.Party) {
			return "", nil
		}
		member := s.Game.Party[i]
		canCast := game.CanCast(member.Class) && !member.Empty()
		if s.Game.Fight != nil {
			canCast = canCast && member.Condition.Acts() && member.CanCastInCombat()
		} else {
			canCast = canCast && member.CanCastNow()
		}
		return member.Name, []choice{{c.Text("items"), "items", s.Mode == ui.ModeExplore}, {c.Text("cast"), "cast", canCast}, {c.Text("details"), "details", true}}
	case "details":
		out := []choice{}
		for _, line := range s.PartyDetails(c.Selected) {
			out = append(out, choice{line, "", false})
		}
		return s.Game.Party[c.Selected].Name, out
	case "read":
		return c.Text("read"), nil
	case "more":
		ids := []string{"save", "world", "cast", "rest", "bash", "unlock", "shop", "create", "help", "settings", "locale", "graphics"}
		if s.Game.Fight != nil {
			ids = []string{"shoot", "run", "quick", "exchange", "protect", "view", "help", "locale", "graphics"}
		}
		out := []choice{}
		for _, id := range ids {
			out = append(out, choice{c.Text(id), id, true})
		}
		return c.Text("more"), out
	case "classes":
		out := []choice{}
		for i := 1; i <= 8; i++ {
			out = append(out, choice{strconv.Itoa(i) + " " + game.Class(i-1).String(), fmt.Sprintf("class:%d", i), s.New.Eligible(game.Class(i - 1))})
		}
		return c.Text("classes"), out
	}
	if s.Mode == ui.ModeMenu && s.Menu != nil {
		out := make([]choice, len(s.Menu.Items))
		for i, label := range s.Menu.Items {
			out[i] = choice{label, fmt.Sprintf("menu:%d", i), true}
		}
		return s.Menu.Title, out
	}
	return "", nil
}

func (c *Controller) Targets() []Target {
	s := c.Session
	out := []Target{}
	add := func(id, icon string, b image.Rectangle, enabled, repeat bool) {
		out = append(out, Target{id, c.Text(id), icon, b, enabled, repeat})
	}
	explore := s.Mode == ui.ModeExplore && c.Overlay == ""
	for _, v := range []struct {
		id, icon string
		x, y     int
	}{{"forward", "up", 73, 126}, {"left", "left", 17, 182}, {"right", "right", 129, 182}, {"backward", "down", 73, 238}} {
		add(v.id, v.icon, image.Rect(v.x, v.y, v.x+52, v.y+52), explore, true)
	}
	for i := range s.Game.Party {
		if i >= 6 {
			break
		}
		x := 188 + i*78
		out = append(out, Target{fmt.Sprintf("party:%d", i), s.Game.Party[i].Name, "", image.Rect(x, 319, x+74, 369), (s.Mode == ui.ModeExplore || s.Mode == ui.ModeCombat) && c.Overlay == "", false})
	}
	ids := []string{"search", "items", "map", "party", "more", "back"}
	icons := []string{"search", "items", "map", "party", "more", "back"}
	switch {
	case c.Overlay != "":
		ids = []string{"confirm", "back", "prev", "next"}
		icons = []string{"check", "back", "up", "down"}
	case s.Mode == ui.ModeCombat:
		ids = []string{"attack", "cast", "block", "items", "more", "back"}
		icons = []string{"sword", "magic", "party", "items", "more", "back"}
	case s.Mode == ui.ModeMenu:
		ids = []string{"confirm", "back", "prev", "next"}
		icons = []string{"check", "back", "up", "down"}
		if s.ItemsOpen() {
			ids = append(ids, "use")
			icons = append(icons, "items")
		} else if s.Message() != "" {
			ids = append(ids, "read")
			icons = append(icons, "more")
		}
	case s.Mode == ui.ModeCreate:
		ids = []string{"classes", "reroll", "exchange_attr", "attr_up", "attr_down", "back"}
		icons = []string{"party", "more", "more", "up", "down", "back"}
	case s.Mode == ui.ModeName || s.Mode == ui.ModeText:
		ids = []string{"input", "back"}
		icons = []string{"check", "back"}
	case s.Mode == ui.ModeMessage:
		ids = []string{"confirm", "back"}
		icons = []string{"check", "back"}
		if p := s.Game.World.Pending; p != nil && p.Kind == game.PromptYesNo {
			ids = []string{"yes", "no", "confirm"}
			icons = []string{"check", "back", "more"}
		}
	case s.Mode == ui.ModeIntro || s.Mode == ui.ModeDead:
		ids = []string{"confirm"}
		icons = []string{"check"}
	case s.Mode == ui.ModeControl:
		ids = []string{"confirm", "back"}
		icons = []string{"check", "back"}
		if _, ok := s.InputValue(); ok {
			ids[0] = "input"
		}
	case s.Mode == ui.ModeMap || s.Mode == ui.ModeWorld:
		ids = []string{"back", "zoom", "prev", "next"}
		icons = []string{"back", "search", "left", "right"}
	}
	if c.isTextPage() && c.textPages() > 1 {
		for _, id := range []string{"prev", "next"} {
			found := false
			for _, existing := range ids {
				if existing == id {
					found = true
				}
			}
			if !found {
				ids = append(ids, id)
				icon := "up"
				if id == "next" {
					icon = "down"
				}
				icons = append(icons, icon)
			}
		}
	}
	for i, id := range ids {
		x, y := 673+(i%2)*81, 99+(i/2)*70
		enabled := true
		if id == "confirm" && c.Overlay != "" {
			enabled = false
		}
		if id == "prev" || id == "next" {
			if c.isTextPage() {
				if id == "prev" {
					enabled = c.Page > 0
				} else {
					enabled = c.Page+1 < c.textPages()
				}
			} else {
				_, options := c.choices()
				if len(options) > 0 {
					if id == "prev" {
						enabled = c.Page > 0
					} else {
						enabled = (c.Page+1)*3 < len(options)
					}
				}
			}
		}
		if id == "items" && s.Mode == ui.ModeCombat {
			enabled = false
		}
		add(id, icons[i], image.Rect(x, y, x+73, y+60), enabled, false)
		if id == "confirm" && s.ItemsOpen() {
			out[len(out)-1].Label = c.Text("equip")
		}
	}
	title, opts := c.choices()
	_ = title
	if len(opts) > 0 {
		if c.Overlay == "classes" {
			for i, o := range opts {
				x, y := 208+(i%2)*216, 92+(i/2)*52
				out = append(out, Target{o.action, o.label, "", image.Rect(x, y, x+208, y+52), o.enabled, false})
			}
		} else {
			page := max(0, min(c.Page, (len(opts)-1)/3))
			for j := 0; j < 3; j++ {
				i := page*3 + j
				if i >= len(opts) {
					break
				}
				o := opts[i]
				out = append(out, Target{o.action, o.label, "", image.Rect(208, 100+j*56, 636, 148+j*56), o.enabled, false})
			}
		}
	}
	return out
}

// Touch coordinates are logical. Only the first finger owns a gesture; input
// is cancelled across a mode/menu/overlay change, including held repetitions.
func (c *Controller) Touch(id, x, y int, down bool, now time.Duration) {
	if down && c.owner == -1 {
		for _, t := range c.Targets() {
			if t.Enabled && image.Pt(x, y).In(c.HitBox(t)) {
				c.owner = id
				c.pressed = t.ID
				c.stamp = c.signature()
				c.started = now
				c.next = now + 400*time.Millisecond
				c.repeated = false
				c.cancelled = false
				c.Dirty = true
				return
			}
		}
		return
	}
	if id != c.owner {
		return
	}
	var selected *Target
	for _, t := range c.Targets() {
		if t.ID == c.pressed {
			v := t
			selected = &v
			break
		}
	}
	if selected == nil || !selected.Enabled || c.signature() != c.stamp || !image.Pt(x, y).In(c.HitBox(*selected)) {
		c.cancelled = true
	}
	if down {
		if !c.cancelled && !c.repeated && strings.HasPrefix(selected.ID, "menu:") && now >= c.next && c.Session.Menu != nil {
			index, _ := strconv.Atoi(strings.TrimPrefix(selected.ID, "menu:"))
			if index >= 0 && index < len(c.Session.Menu.Items) {
				c.Session.Menu.Cur = index
				c.repeated = true
				c.Dirty = true
			}
		}
		if !c.cancelled && selected.Repeat && now >= c.next {
			c.repeated = true
			c.next = now + 180*time.Millisecond
			c.Act(selected.ID)
			if c.signature() != c.stamp {
				c.cancelled = true
			}
		}
		return
	}
	act := c.pressed
	ok := !c.cancelled && !c.repeated
	c.owner = -1
	c.pressed = ""
	c.Dirty = true
	if ok {
		c.Act(act)
	}
}

func (c *Controller) Act(id string) {
	c.ActionSerial++
	c.LastAction = id
	s := c.Session
	fromParty := c.Overlay == "party"
	c.Dirty = true
	c.save()
	if strings.HasPrefix(id, "party:") {
		i, _ := strconv.Atoi(strings.TrimPrefix(id, "party:"))
		if s.SelectParty(i) {
			c.Selected = i
			c.Overlay = "party"
			c.Page = 0
		}
		return
	}
	if strings.HasPrefix(id, "menu:") {
		i, _ := strconv.Atoi(strings.TrimPrefix(id, "menu:"))
		if s.ItemsOpen() {
			if i >= 0 && i < len(s.Menu.Items) {
				s.Menu.Cur = i
			}
			return
		}
		s.SelectMenu(i)
		c.Page = 0
		c.save()
		return
	}
	if strings.HasPrefix(id, "class:") {
		i, _ := strconv.Atoi(strings.TrimPrefix(id, "class:"))
		s.CreatePickClass(i)
		c.Overlay = ""
		c.Page = 0
		return
	}
	switch id {
	case "party":
		c.Selected = 0
		c.Overlay = "party"
		c.Page = 0
		return
	case "more":
		c.Overlay = "more"
		c.Page = 0
		return
	case "classes":
		c.Overlay = "classes"
		c.Page = 0
		return
	case "details":
		c.Overlay = "details"
		c.Page = 0
		return
	case "read":
		c.Overlay = "read"
		c.Page = 0
		return
	case "back":
		if c.Overlay != "" {
			c.Overlay = ""
			c.Page = 0
			return
		}
	case "prev", "next":
		delta := 1
		if id == "prev" {
			delta = -1
		}
		if c.isTextPage() {
			c.Page = max(0, min(c.Page+delta, c.textPages()-1))
			return
		}
		_, opts := c.choices()
		if len(opts) > 0 {
			c.Page = max(0, min(c.Page+delta, (len(opts)-1)/3))
			return
		}
		if s.Mode == ui.ModeWorld {
			s.Key(ui.KeyLeft)
		} else if s.Mode == ui.ModeMap {
			if delta < 0 {
				s.Key(ui.KeyUp)
			} else {
				s.Key(ui.KeyDown)
			}
		}
		return
	case "zoom":
		c.MapZoom = !c.MapZoom
		return
	case "locale":
		p := s.Presentation()
		if p.Locale == i18n.Traditional {
			p.Locale = i18n.Simplified
		} else {
			p.Locale = i18n.Traditional
		}
		if err := s.SetPresentation(p); err != nil {
			c.Status = err.Error()
		}
		c.Overlay = ""
		return
	case "graphics":
		p := s.Presentation()
		p.HD = !p.HD
		if err := s.SetPresentation(p); err != nil {
			c.Status = err.Error()
		}
		c.Overlay = ""
		return
	case "input":
		c.InputToken++
		c.EditorRequested = true
		return
	case "confirm":
		if c.isTextPage() && c.Page+1 < c.textPages() {
			c.Page++
			return
		}
		if c.Overlay != "" {
			return
		}
	}
	keys := map[string]ui.Key{"forward": ui.KeyForward, "backward": ui.KeyBack, "left": ui.KeyLeft, "right": ui.KeyRight, "search": ui.KeySearch, "items": ui.KeyItems, "map": ui.KeyMap, "world": ui.KeyWorld, "attack": ui.KeyConfirm, "confirm": ui.KeyConfirm, "cast": ui.KeyCast, "block": ui.KeyBlock, "back": ui.KeyCancel, "yes": ui.KeyYes, "no": ui.KeyNo, "rest": ui.KeyRest, "bash": ui.KeyBash, "unlock": ui.KeyUnlock, "shop": ui.KeyShop, "create": ui.KeyCreate, "help": ui.KeyHelp, "settings": ui.KeySettings, "save": ui.KeySave, "shoot": ui.KeyShoot, "run": ui.KeyRun, "quick": ui.KeyQuickFight, "exchange": ui.KeyExch, "protect": ui.KeyProt, "view": ui.KeyView, "use": ui.KeyUse, "reroll": ui.KeyConfirm, "exchange_attr": ui.KeyUse, "attr_up": ui.KeyUp, "attr_down": ui.KeyDown}
	if key, ok := keys[id]; ok {
		c.Overlay = ""
		c.Page = 0
		s.Key(key)
		if id == "cast" && fromParty && s.Mode == ui.ModeMenu {
			for i, member := range s.Casters() {
				if member == c.Selected {
					s.SelectMenu(i)
					break
				}
			}
		}
		if _, active := s.InputValue(); active {
			c.InputToken++
		}
		c.save()
	}
}

func (c *Controller) Submit(text string, token int, confirm bool) bool {
	if token != c.InputToken {
		return false
	}
	if _, active := c.Session.InputValue(); !active {
		return false
	}
	if confirm {
		before, _ := c.Session.InputValue()
		expected := text
		if c.Session.Mode == ui.ModeText || c.Session.Mode == ui.ModeControl {
			expected = i18n.NormalizeInputWidth(text)
		}
		if len([]rune(expected)) > c.Session.InputLimit() {
			c.Status = fmt.Sprintf(c.Text("input_too_long"), c.Session.InputLimit())
			c.Dirty = true
			return false
		}
		c.Session.ReplaceInput(text)
		value, _ := c.Session.InputValue()
		if value != expected {
			c.Session.ReplaceInput(before)
			c.Status = c.Text("input_invalid")
			c.Dirty = true
			return false
		}
		c.Session.Key(ui.KeyConfirm)
		if c.Session.Mode == ui.ModeName {
			c.Session.ReplaceInput(before)
			c.Status = c.Text("input_required")
			c.Dirty = true
			return false
		}
		c.Status = ""
		c.InputToken++
	}
	c.EditorRequested = false
	c.Dirty = true
	c.save()
	return true
}

func (c *Controller) Input() (string, bool) { return c.Session.InputValue() }

func (c *Controller) isTextPage() bool {
	s := c.Session
	_, ok := s.MobileTextPage()
	return ok || s.Mode == ui.ModeMessage || s.Mode == ui.ModeText || c.Overlay == "read"
}
func (c *Controller) textContent() (string, []string) {
	s := c.Session
	title := c.Text("message")
	body := s.Message()
	if c.Overlay == "read" {
		title = c.Text("read")
	} else if page, ok := s.MobileTextPage(); ok {
		title = page.Title + " " + page.Clock
		body = strings.Join(page.Lines, "@")
		if page.Prompt != "" {
			body += "@" + page.Prompt + page.Input
		}
	} else if s.Mode == ui.ModeText {
		title = c.Text("input_title")
		body += "@" + c.Text("input_instruction")
	}
	return title, c.wrap(body, 428, 14)
}
func (c *Controller) textPages() int { _, lines := c.textContent(); return max(1, (len(lines)+9)/10) }
func (c *Controller) wrap(text string, width, size int) []string {
	out := []string{}
	line := ""
	used := 0
	for _, r := range c.Convert(text) {
		if r == '@' || r == '\n' {
			out = append(out, line)
			line = ""
			used = 0
			continue
		}
		advance := size * 2
		if r < 128 {
			advance = size
		}
		if used+advance > width*2 && line != "" {
			out = append(out, line)
			line = ""
			used = 0
		}
		line += string(r)
		used += advance
	}
	if line != "" || len(out) == 0 {
		out = append(out, line)
	}
	return out
}
