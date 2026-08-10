package view

import (
	"strings"
	"testing"
)

// width 是一行畫出來會佔幾個原版像素。中文與 ASCII 一樣寬，見 GlyphCols。
func width(s string) int { return len([]rune(s)) * GlyphCols }

func TestWrapFitsLimit(t *testing.T) {
	cases := []string{
		"由柯拉克蒐集，昆登在皇家豪華宮殿（Luxus Palace Royale）的柯拉克研究室中找到。",
		"顯示施法者背包中具有魔法之物品，並標示使用次數。",
		"Supercalifragilisticexpialidocious",
		"",
	}
	for _, limit := range []int{64, 96, 200, 303} {
		for _, c := range cases {
			for _, line := range wrap(c, limit) {
				if width(line) > limit {
					t.Errorf("limit=%d 折出來的一行寬 %d：%q",
						limit, width(line), line)
				}
			}
		}
	}
}

func TestWrapKeepsWordsWhole(t *testing.T) {
	// 這一句擺在 96 px（十二格）寬的框裡一定會折到，
	// 而 Luxus / Palace / Royale 三個字都放得下，所以一個都不該被切開。
	got := wrap("昆登在皇家豪華宮殿（Luxus Palace Royale）中", 96)
	joined := strings.Join(got, "|")
	for _, word := range []string{"Luxus", "Palace", "Royale"} {
		if !strings.Contains(joined, word) {
			t.Errorf("%q 被切開了：%q", word, got)
		}
	}
}

func TestWrapSplitsOverlongWord(t *testing.T) {
	// 比整行還長的英文字沒得斷，只能硬切 —— 但不能因此丟字。
	const word = "Supercalifragilisticexpialidocious"
	got := wrap(word, 80)
	if len(got) < 2 {
		t.Fatalf("整個字塞進一行了：%q", got)
	}
	if j := strings.Join(got, ""); j != word {
		t.Errorf("硬切之後字變了：%q", j)
	}
}

func TestWrapPunctuationNotAtLineStart(t *testing.T) {
	// 每個字都恰好一格，湊成「折點正好落在句號前面」的情形。
	got := wrap("天地之間原本只有太虛。虛無中出現能發展生命的物質。", 96)
	if len(got) < 2 {
		t.Fatalf("沒有折行，測不到禁則：%q", got)
	}
	for _, line := range got[1:] {
		r := []rune(line)
		if len(r) > 0 && noLineStart(string(r[0])) {
			t.Errorf("句讀排到行首了：%q（全部：%q）", line, got)
		}
	}
}

func TestWrapKeepsEmptyLine(t *testing.T) {
	// 空行是排版的一部分（章節之間的間隔），不能被吃掉。
	if got := wrap("", 100); len(got) != 1 || got[0] != "" {
		t.Errorf("空行被吃掉了：%q", got)
	}
}

func TestWrapBracketNotAtLineEnd(t *testing.T) {
	// 開括號留在行尾，括住的內容卻跑到下一行 —— 讀起來是斷的。
	got := wrap("昆登在皇家豪華宮殿（Luxus Palace Royale）的柯拉克研究室中找到。", 200)
	if len(got) < 2 {
		t.Fatalf("沒有折行，測不到禁則：%q", got)
	}
	for _, line := range got[:len(got)-1] {
		r := []rune(strings.TrimRight(line, " "))
		if len(r) > 0 && noLineEnd(string(r[len(r)-1])) {
			t.Errorf("開括號留在行尾了：%q（全部：%q）", line, got)
		}
	}
}

func TestWrapKeepsLeadingIndent(t *testing.T) {
	// 選單未選中的項目前面是兩個空白，靠它與選中那一項（「▶ 」）對齊。
	// 折行時把行首空白一律吃掉，整欄就會參差不齊。
	got := wrap("  2) 城鎮指令", 200)
	if !strings.HasPrefix(got[0], "  2)") {
		t.Errorf("原文的縮排被吃掉了：%q", got)
	}
	// 續行則不該留下折點處的空白。
	long := wrap("  1) "+strings.Repeat("word ", 20), 200)
	for _, line := range long[1:] {
		if strings.HasPrefix(line, " ") {
			t.Errorf("續行留了行首空白：%q", line)
		}
	}
}
