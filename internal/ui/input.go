package ui

// InputValue returns committed text only. Preedit never reaches the game/save.
func (s *Session) InputValue() (string, bool) {
	p := s.inputBuffer()
	if p == nil {
		return "", false
	}
	return *p, true
}

func (s *Session) inputBuffer() *string {
	switch s.Mode {
	case ModeName:
		return &s.New.Name
	case ModeText:
		return &s.PromptText
	case ModeControl:
		if s.controlStage == stageAbort || s.controlStage == stageCipher {
			return &s.controlInput
		}
	}
	return nil
}

// ReplaceInput applies the same validation and length limits as typed runes.
func (s *Session) ReplaceInput(text string) bool {
	p := s.inputBuffer()
	if p == nil {
		return false
	}
	before := *p
	*p = ""
	feedback := ""
	for _, r := range text {
		if r == '\b' {
			continue
		}
		s.TypeRune(r)
		if s.presentation.inputError != "" {
			feedback = s.presentation.inputError
		}
	}
	s.presentation.inputError = feedback
	return *p != before
}

func (s *Session) SetInputPreview(text string, active bool) bool {
	p := &s.presentation
	changed := p.inputPreview != text || p.inputPreviewActive != active
	p.inputPreview, p.inputPreviewActive = text, active
	return changed
}

func (s *Session) inputForDisplay(committed string) string {
	if s.presentation.inputPreviewActive {
		return s.presentation.inputPreview
	}
	return committed
}
