package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func fixtureEXE() []byte {
	maxDS := tunePointers + 10*2
	for _, ds := range []int{pitchTable + 2*0x07, durationTable + 2*0x05, 0x5104} {
		if ds+2 > maxDS {
			maxDS = ds + 2
		}
	}
	exe := make([]byte, dgroupBase+maxDS+16)
	putWord := func(ds, value int) { binary.LittleEndian.PutUint16(exe[dgroupBase+ds:], uint16(value)) }
	putWord(pitchTable+2*0x03, 262)
	putWord(pitchTable+2*0x07, 330)
	putWord(durationTable+2*0x02, 500)
	putWord(durationTable+2*0x05, 62)
	putWord(tunePointers+3*2, 0x5100)
	exe[dgroupBase+0x5100] = 0x03
	exe[dgroupBase+0x5101] = 0x02
	exe[dgroupBase+0x5102] = 0x07
	exe[dgroupBase+0x5103] = 0x05
	exe[dgroupBase+0x5104] = 0xff
	return exe
}

func TestReadTuneUsesDGROUPTablesAndTerminator(t *testing.T) {
	notes, err := readTune(fixtureEXE(), 3)
	if err != nil {
		t.Fatalf("readTune: %v", err)
	}
	if len(notes) != 2 || notes[0] != (note{hz: 262, ticks: 500}) || notes[1] != (note{hz: 330, ticks: 62}) {
		t.Fatalf("notes = %#v, want two table-derived notes", notes)
	}
	if got := duration(notes); got != float64(562*pitDivisor)/pitClockHz {
		t.Fatalf("duration = %v, want PIT-derived duration", got)
	}
}

func TestVerifyMM2EXERejectsNonOracleData(t *testing.T) {
	if err := verifyMM2EXE(fixtureEXE()); err == nil {
		t.Fatal("verifyMM2EXE accepted synthetic data")
	}
}

func TestReadTuneRejectsInvalidIndexAndMissingTerminator(t *testing.T) {
	bad := fixtureEXE()
	if _, err := readTune(bad, 10); err == nil {
		t.Fatal("readTune accepted invalid tune index")
	}
	for i := 0; i < 1024; i++ {
		pos := dgroupBase + 0x5100 + i*2
		if pos+1 >= len(bad) {
			break
		}
		bad[pos], bad[pos+1] = 0x03, 0x02
	}
	if _, err := readTune(bad, 3); err == nil {
		t.Fatal("readTune accepted a sequence without terminator")
	}
}

func TestParseTunesRejectsEmptyAndOutOfRange(t *testing.T) {
	for _, input := range []string{"", "3,,4", "10", "-1", "x"} {
		if _, err := parseTunes(input); err == nil {
			t.Errorf("parseTunes(%q) accepted invalid input", input)
		}
	}
	got, err := parseTunes(" 3, 4 ")
	if err != nil || len(got) != 2 || got[0] != 3 || got[1] != 4 {
		t.Fatalf("parseTunes valid input = %#v, %v", got, err)
	}
}

func TestWriteWAVHeaderAndLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "test.wav")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	pcm := []int16{7200, -7200, 0}
	if err := writeWAV(path, pcm); err != nil {
		t.Fatalf("writeWAV: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 44+len(pcm)*2 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" || string(b[36:40]) != "data" {
		t.Fatalf("invalid WAV header or size: len=%d", len(b))
	}
	if got := binary.LittleEndian.Uint32(b[40:44]); got != uint32(len(pcm)*2) {
		t.Fatalf("data size = %d, want %d", got, len(pcm)*2)
	}
}

func TestRenderLoopHonorsRequestedLengthAndSilence(t *testing.T) {
	pcm := renderLoop([]note{{hz: 1000, ticks: 100}, {silenceSamples: 1}}, 1)
	if len(pcm) != sampleRate {
		t.Fatalf("rendered samples = %d, want %d", len(pcm), sampleRate)
	}
	if pcm[0] == 0 || pcm[100*pitDivisor*sampleRate/pitClockHz] != 0 {
		t.Fatal("renderLoop did not render tone followed by silence")
	}
}

func TestDurationUsesPITTickClock(t *testing.T) {
	got := duration([]note{{ticks: 1193182}})
	want := float64(pitDivisor)
	if got != want {
		t.Fatalf("duration = %v, want %v seconds", got, want)
	}
}
