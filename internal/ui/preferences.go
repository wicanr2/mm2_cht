package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func PresentationPath() string {
	root, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(root, "mm2-cht", "presentation.json")
}

func ReadPresentation(path string) (Presentation, error) {
	p := DefaultPresentation()
	blob, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(blob, &p)
	return p, err
}

func WritePresentation(path string, p Presentation) error {
	if path == "" {
		return nil
	}
	blob, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(blob, '\n'), 0600)
}

func writeFileAtomic(path string, blob []byte, mode os.FileMode) error {
	var err error
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mm2-save-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err = tmp.Write(blob); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
