package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/music"
)

// Release verification uses the real encoded pack without opening an audio device.
func TestReleaseMusicDecodeAndLoopSeek(t *testing.T) {
	path := os.Getenv("MM2_VERIFY_MUSIC")
	if path == "" {
		t.Skip("release music fixture not requested")
	}
	pack, err := music.LoadManifest(path, music.ThemeMegaDrive)
	if err != nil {
		t.Fatal(err)
	}
	for role, path := range pack.Tracks {
		t.Run(string(role), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			stream, err := (localTrack{ext: filepath.Ext(path), data: data}).decode()
			if err != nil {
				t.Fatal(err)
			}
			n, err := io.Copy(io.Discard, stream)
			if err != nil {
				t.Fatal(err)
			}
			if n <= 0 || n%4 != 0 {
				t.Fatalf("invalid stereo PCM length %d", n)
			}
			if _, err = stream.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			if _, err = io.ReadFull(stream, make([]byte, 4096)); err != nil {
				t.Fatalf("loop rewind: %v", err)
			}
		})
	}
}
