package cjk

import (
	"encoding/binary"
	"testing"
)

func TestAlphaAtlasCoverageAndBounds(t *testing.T) {
	b := append([]byte("MM2CJA\x00\x00"), make([]byte, 8)...)
	binary.LittleEndian.PutUint16(b[8:], 2)
	binary.LittleEndian.PutUint16(b[10:], 2)
	binary.LittleEndian.PutUint32(b[12:], 1)
	b = append(b, 'A', 0, 0, 0, 0, 64, 128, 255)
	f, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if f.Coverage('A', 1, 0) != 64 || f.Coverage('A', 0, 1) != 128 || f.Coverage('A', 1, 1) != 255 || f.Coverage('B', 0, 0) != 0 {
		t.Fatal("alpha coverage lost")
	}
	for _, blob := range [][]byte{b[:len(b)-1], append(append([]byte{}, b...), 0)} {
		if _, err := Parse(blob); err == nil {
			t.Fatal("invalid length accepted")
		}
	}
	binary.LittleEndian.PutUint16(b[8:], 0)
	if _, err := Parse(b); err == nil {
		t.Fatal("zero width accepted")
	}
}
