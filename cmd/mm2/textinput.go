package main

import (
	"image"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/wicanr2/mm2_cht/internal/render"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

func (a *app) updateTextInput() bool {
	value, active := a.sess.InputValue()
	if !active {
		if a.inputActive {
			a.inputField.Blur()
			a.inputActive = false
		}
		if a.sess.SetInputPreview("", false) {
			a.dirty = true
		}
		if a.sess.Mode == ui.ModeControl {
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && a.sess.Key(ui.KeyConfirm) {
				a.dirty = true
			}
			return true
		}
		return false
	}
	if !a.inputActive || a.inputMode != a.sess.Mode {
		a.inputField.Blur()
		a.inputField = textinput.Field{}
		a.inputField.Focus()
		a.inputActive, a.inputMode = true, a.sess.Mode
		a.inputField.SetTextAndSelection(value, len(value), len(value))
	} else if a.inputField.Text() != value {
		a.inputField.SetTextAndSelection(value, len(value), len(value))
	}
	rowHeight := 0
	if a.toolbar != nil {
		rowHeight = a.toolbar.RowHeight()
	}
	w, h := max(1, a.width), max(1, a.height)
	scale, ox, oy := render.FitCanvas(w, h, render.HiW, render.HiH+rowHeight)
	x, y := int(math.Round(ox+72*scale)), int(math.Round(oy+float64(rowHeight+132)*scale))
	handled, err := a.inputField.HandleInputWithBounds(image.Rect(x, y, x+1, y+max(1, int(24*scale))))
	if err != nil {
		log.Printf("text input: %v", err)
		a.inputField.Blur()
		a.inputActive = false
		return true
	}
	if committed := a.inputField.Text(); committed != value {
		if a.sess.ReplaceInput(committed) {
			a.dirty = true
		}
		clean, _ := a.sess.InputValue()
		if clean != committed {
			a.inputField.SetTextAndSelection(clean, len(clean), len(clean))
		}
	}
	composing := a.inputField.UncommittedTextLengthInBytes() > 0
	if a.sess.SetInputPreview(a.inputField.TextForRendering(), composing) {
		a.dirty = true
	}
	// Enter/Esc used to choose or cancel IME composition cannot submit game text.
	if handled || composing {
		return true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && a.sess.TypeRune('\b') {
		a.dirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && a.sess.Key(ui.KeyConfirm) {
		a.dirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && a.sess.Key(ui.KeyCancel) {
		a.dirty = true
	}
	return true
}
