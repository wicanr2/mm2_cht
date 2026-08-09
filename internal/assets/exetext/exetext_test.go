// 這些測試拿原版 MM2.EXE 當對照。原版資料不入版控，找不到就 skip。
package exetext_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/exetext"
)

func orig(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "workplace", "orig", "MM2", "MM2.EXE")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("找不到 %s（玩家自備合法原版，解到 workplace/orig/）", path)
	}
	return b
}

// DGROUP 初值段的位移是整個抽取的前提。拿幾條已知位置的字串釘住它 ——
// 位移一旦飄掉，這裡會先炸，而不是等到譯文檔整批對不上 key。
func TestKnownOffsets(t *testing.T) {
	exe := orig(t)
	for off, want := range map[int]string{
		0x000F: "Middlegate",   // 起始城鎮
		0x003E: "Knight",       // 第一個職業
		0x0E2C: "sprays poison", // 第一種特殊攻擊
		0x4318: "Might",        // 第一個屬性
	} {
		got, err := exetext.At(exe, off)
		if err != nil {
			t.Errorf("ds:%04X: %v", off, err)
			continue
		}
		if got != want {
			t.Errorf("ds:%04X = %q，預期 %q", off, got, want)
		}
	}
}

// 檔名不該進待譯清單 —— 翻了遊戲就開不了檔。
func TestFilenamesExcluded(t *testing.T) {
	exe := orig(t)
	all, err := exetext.Parse(exe)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 500 {
		t.Fatalf("只抽出 %d 條字串，位移可能不對", len(all))
	}
	for _, s := range all {
		switch s.Text {
		case "monsters.16", "eventsi.dat", "map.dat", "attrib.dat", "str.dat":
			t.Errorf("檔名 %q 不該出現在待譯清單裡（%s）", s.Text, s.Key())
		}
	}
}

// key 用 DGROUP 偏移，抽取規則調整時不會整批位移。
func TestKeyFormat(t *testing.T) {
	if got := (exetext.String{Offset: 0x0E2C}).Key(); got != "exe.0E2C" {
		t.Errorf("Key() = %q，預期 exe.0E2C", got)
	}
}
