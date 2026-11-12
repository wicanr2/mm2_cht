//go:build android

// Package mobile is the thin Ebitengine/Android binding. The existing UI owns
// gameplay; Android supplies private paths, lifecycle and a native IME editor.
package mobile

import (
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/mobile"
	"github.com/wicanr2/mm2_cht/internal/mobileui"
	gamemusic "github.com/wicanr2/mm2_cht/internal/music"
	"github.com/wicanr2/mm2_cht/internal/ui"
	"golang.org/x/sys/unix"
)

var gameApp = &app{}
var Version = "development"

type pointerEvent struct {
	id, x, y, action int
	at               time.Duration
}

var pointerMu sync.Mutex
var pointerQueue []pointerEvent
var requestSnapshot, diagnosticSnapshot, errorSnapshot atomic.Value
var viewportPending [2]float64
var viewportChanged bool

func SetDisplayDensity(density float64) { mobileui.Scale = max(2, min(3, int(math.Ceil(density)))) }

type touchPoint struct{ x, y int }
type app struct {
	mu                sync.Mutex
	c                 *mobileui.Controller
	frame             *ebiten.Image
	music             *musicPlayer
	w, h              int
	frames            int
	frameSerial       int64
	pointerSerial     int64
	lastPointer       pointerEvent
	paused            bool
	last              map[int]touchPoint
	inputSeen         string
	dpWidth, dpHeight float64
}

func init() { mobile.SetGame(gameApp) }

func Initialize(runtimeRoot, playerRoot string) error {
	a := gameApp
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c != nil {
		return nil
	}
	if !filepath.IsAbs(runtimeRoot) || !filepath.IsAbs(playerRoot) {
		return fmt.Errorf("private paths must be absolute")
	}
	if err := os.MkdirAll(playerRoot, 0700); err != nil {
		return err
	}
	for _, name := range []string{"assets", "translations", "data", "workplace"} {
		target := filepath.Join(playerRoot, name)
		source := filepath.Join(runtimeRoot, name)
		if old, err := os.Readlink(target); err == nil {
			if old == source {
				continue
			}
			if err := os.Remove(target); err != nil {
				return err
			}
		}
		if err := os.Symlink(source, target); err != nil {
			return err
		}
	}
	if err := os.Chdir(playerRoot); err != nil {
		return err
	}
	os.Setenv("MM2_ASSET_ROOT", filepath.Join(runtimeRoot, "workplace"))
	s, err := ui.LoadWithOptions(filepath.Join(runtimeRoot, "workplace/orig/MM2"), ui.LoadOptions{Theme: "amiga"})
	if err != nil {
		return err
	}
	restored := s.Restore()
	if err = s.InitPresentation(runtimeRoot, filepath.Join(runtimeRoot, "workplace/hd-amiga-r1"), filepath.Join(runtimeRoot, "workplace/amiga")); err != nil {
		return err
	}
	p, err := ui.ReadPresentation(filepath.Join(playerRoot, "presentation.json"))
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(playerRoot, "presentation.json")); os.IsNotExist(err) {
		p.HD = true
		p.Platform = "amiga"
	}
	if err = s.SetPresentation(p); err != nil {
		return err
	}
	f, err := os.Open(filepath.Join(runtimeRoot, "workplace/hd-amiga-r1/frame.png"))
	if err != nil {
		return err
	}
	art, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	a.c = mobileui.New(s, art)
	a.last = map[int]touchPoint{}
	if !restored {
		s.ShowIntro()
	}
	if pack, err := gamemusic.LoadManifest(filepath.Join(runtimeRoot, "music/manifest.json"), gamemusic.ThemeMegaDrive); err == nil {
		a.music, err = newMusicPlayer(pack)
		if err != nil {
			a.c.Status = err.Error()
		}
	}
	return nil
}

func (a *app) Update() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c == nil || a.paused {
		return nil
	}
	presentationBefore := a.c.Session.Presentation()
	a.frames++
	pointerMu.Lock()
	events := pointerQueue
	pointerQueue = nil
	if viewportChanged {
		a.dpWidth, a.dpHeight = viewportPending[0], viewportPending[1]
		viewportChanged = false
		f := math.Min(a.dpWidth/mobileui.Width, a.dpHeight/mobileui.Height)
		if f > 0 {
			a.c.MinHit = int(math.Ceil(48 / f))
		}
		a.c.Dirty = true
	}
	pointerMu.Unlock()
	var clock unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &clock); err != nil {
		return fmt.Errorf("Android input clock: %w", err)
	}
	now := time.Duration(clock.Nano())
	for _, event := range events {
		a.pointerSerial++
		a.lastPointer = event
		p := touchPoint{event.x, event.y}
		occurred := event.at
		switch event.action {
		case 0:
			a.last[event.id] = p
			a.c.Touch(event.id, p.x, p.y, true, occurred)
		case 1:
			a.last[event.id] = p
			if a.c.Owner() == event.id {
				a.c.Touch(event.id, p.x, p.y, true, occurred)
			}
		case 2:
			a.c.Touch(event.id, p.x, p.y, false, occurred)
			delete(a.last, event.id)
		case 3:
			a.c.CancelTouch()
			a.last = map[int]touchPoint{}
		}
	}
	if p, ok := a.last[a.c.Owner()]; ok {
		a.c.Touch(a.c.Owner(), p.x, p.y, true, now)
	}
	if after := a.c.Session.Presentation(); after != presentationBefore {
		if err := ui.WritePresentation("presentation.json", after); err != nil {
			a.c.Status = err.Error()
		}
	}
	if a.frames%8 == 0 {
		a.c.Session.Tick()
		a.c.Dirty = true
	}
	if value, ok := a.c.Input(); ok {
		stamp := fmt.Sprintf("%d/%s", a.c.Session.Mode, value)
		if a.inputSeen == "" {
			a.c.InputToken++
			if a.c.Session.Mode == ui.ModeName {
				a.c.EditorRequested = true
			}
		}
		a.inputSeen = stamp
	} else {
		a.inputSeen = ""
		a.c.EditorRequested = false
	}
	if a.music != nil {
		if err := a.music.Sync(a.c.Session.MusicCue()); err != nil {
			a.c.Status = err.Error()
		}
		if cue, ok := a.c.Session.Stinger(); ok {
			if err := a.music.PlayOnce(cue); err != nil {
				a.c.Status = err.Error()
			}
		}
	}
	if a.c.Dirty || a.frame == nil {
		pixels := a.c.Draw()
		if a.frame == nil {
			a.frame = ebiten.NewImageFromImage(pixels)
		} else {
			a.frame.WritePixels(pixels.Pix)
		}
		a.c.Dirty = false
		a.frameSerial++
		a.publish()
	} else if a.frames%60 == 0 {
		a.publish()
	}
	return nil
}
func (a *app) fit() (float64, float64, float64) {
	w, h := max(1, a.w), max(1, a.h)
	f := math.Min(float64(w)/mobileui.Width, float64(h)/mobileui.Height)
	return f, (float64(w) - mobileui.Width*f) / 2, (float64(h) - mobileui.Height*f) / 2
}
func (a *app) Draw(dst *ebiten.Image) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.frame == nil {
		return
	}
	f, x, y := a.fit()
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.GeoM.Scale(f/float64(mobileui.Scale), f/float64(mobileui.Scale))
	op.GeoM.Translate(x, y)
	dst.DrawImage(a.frame, op)
}
func (a *app) Layout(w, h int) (int, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.w, a.h = max(1, w), max(1, h)
	return a.w, a.h
}

func SetViewportDp(width, height float64) {
	pointerMu.Lock()
	viewportPending = [2]float64{width, height}
	viewportChanged = true
	pointerMu.Unlock()
}

// Preserve complete gestures even when rendering is slower than touch events.
func Pointer(id int64, x, y float64, action, eventMillis int64) {
	pointerMu.Lock()
	defer pointerMu.Unlock()
	// Android's event time survives batching. Processing a queued MOVE after a
	// slow render must not turn a 160ms tap into a 400ms long press.
	event := pointerEvent{id: int(id), x: int(math.Floor(x)), y: int(math.Floor(y)), action: int(action), at: time.Duration(eventMillis) * time.Millisecond}
	if len(pointerQueue) >= 256 {
		pointerQueue = []pointerEvent{{action: 3}}
		return
	}
	if action == 1 && len(pointerQueue) > 0 {
		last := &pointerQueue[len(pointerQueue)-1]
		if last.action == 1 && last.id == event.id {
			*last = event
			return
		}
	}
	pointerQueue = append(pointerQueue, event)
}
func CancelPointer() {
	Pointer(0, 0, 0, 3, 0)
}
func Pause() {
	a := gameApp
	a.mu.Lock()
	defer a.mu.Unlock()
	a.paused = true
	pointerMu.Lock()
	pointerQueue = nil
	pointerMu.Unlock()
	a.last = map[int]touchPoint{}
	if a.c != nil {
		a.c.CancelTouch()
		a.c.Checkpoint()
		_ = ui.WritePresentation("presentation.json", a.c.Session.Presentation())
	}
	if a.music != nil && a.music.player != nil {
		a.music.player.Pause()
	}
	a.publish()
}
func Resume() {
	a := gameApp
	a.mu.Lock()
	defer a.mu.Unlock()
	a.paused = false
	pointerMu.Lock()
	pointerQueue = nil
	pointerMu.Unlock()
	a.last = map[int]touchPoint{}
	if a.c != nil {
		a.c.Dirty = true
	}
	if a.music != nil && a.music.player != nil {
		a.music.player.Play()
	}
	a.publish()
}
func Back() {
	a := gameApp
	a.mu.Lock()
	defer a.mu.Unlock()
	pointerMu.Lock()
	pointerQueue = nil
	pointerMu.Unlock()
	a.last = map[int]touchPoint{}
	if a.c != nil {
		a.c.CancelTouch()
		a.c.Act("back")
	}
}

func readSnapshot(value *atomic.Value, fallback string) string {
	if v := value.Load(); v != nil {
		return v.(string)
	}
	return fallback
}
func InputRequest() string { return readSnapshot(&requestSnapshot, "") }
func Diagnostics() string  { return readSnapshot(&diagnosticSnapshot, "{}") }
func InputError() string   { return readSnapshot(&errorSnapshot, "") }
func (a *app) publish() {
	requestSnapshot.Store(a.inputRequest())
	diagnosticSnapshot.Store(a.diagnostics())
	if a.c != nil {
		errorSnapshot.Store(a.c.Status)
	}
}
func (a *app) inputRequest() string {
	if a.c == nil {
		return ""
	}
	if !a.c.EditorRequested {
		return ""
	}
	value, ok := a.c.Input()
	if !ok {
		return ""
	}
	title := a.c.Text("input_title")
	if a.c.Session.Mode == ui.ModeName {
		title = a.c.Text("name_title")
	}
	if a.c.Session.Mode == ui.ModeControl {
		title = a.c.Text("code_title")
	}
	b, _ := json.Marshal(map[string]any{"token": a.c.InputToken, "value": value, "title": title, "confirm": a.c.Text("submit"), "cancel": a.c.Text("cancel")})
	return string(b)
}
func SubmitInput(value string, token int64, confirm bool) bool {
	a := gameApp
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.c != nil {
		ok := a.c.Submit(value, int(token), confirm)
		a.publish()
		return ok
	}
	return false
}
func (a *app) diagnostics() string {
	if a.c == nil {
		return "{}"
	}
	s := a.c.Session
	w := s.Game.World
	targets := a.c.Targets()
	metrics := []map[string]any{}
	f := math.Min(a.dpWidth/mobileui.Width, a.dpHeight/mobileui.Height)
	for _, t := range targets {
		if t.Enabled {
			hit := a.c.HitBox(t)
			metrics = append(metrics, map[string]any{"id": t.ID, "label": t.Label, "box": []int{hit.Min.X, hit.Min.Y, hit.Max.X, hit.Max.Y}, "width_dp": float64(hit.Dx()) * f, "height_dp": float64(hit.Dy()) * f})
		}
	}
	b, _ := json.Marshal(map[string]any{"version": Version, "frame_serial": a.frameSerial, "action_serial": a.c.ActionSerial, "last_action": a.c.LastAction, "pointer_serial": a.pointerSerial, "last_pointer": []int{a.lastPointer.id, a.lastPointer.x, a.lastPointer.y, a.lastPointer.action}, "pointer_owner": a.c.Owner(), "mode": int(s.Mode), "map": w.MapIndex, "x": w.X, "y": w.Y, "face": w.Face, "locale": s.Presentation().Locale, "hd": s.Presentation().HD, "overlay": a.c.Overlay, "viewport_dp": []float64{a.dpWidth, a.dpHeight}, "targets": metrics, "checkpoint_available": s.CanCheckpoint(), "roster_count": len(s.Game.Roster), "paused": a.paused})
	return string(b)
}
