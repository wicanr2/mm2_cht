// mm2data 從原版 MM2.EXE 產生引擎要用的資料檔。
//
//	go run ./cmd/mm2data -exe workplace/orig/MM2/MM2.EXE -out data
//
// 產出四個檔：opcodes.json、combat.json、encounter.json、specials.json。
// 它們是原版資料，不入版控 —— 玩家用自己那份合法原版產生。
//
// 這些表在 IDA 裡看起來是 BSS（`db N dup(?)`），因為 IDA 只載入 MZ image
// 宣告的那 34,320 bytes。實體檔案有 77,824，尾部那 43,472 bytes 就是
// DGROUP 的初值段，對應關係是 `EXE 檔內偏移 = DGROUP 偏移 + 0x8630`。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wicanr2/mm2_cht/internal/assets/exetext"
	"github.com/wicanr2/mm2_cht/internal/gamedata"
)

// DGROUP 偏移。每一個都在反組譯裡看到程式碼以它當基底索引。
const (
	offOpLen         = 0x15E6 // sub_18F64 跳過 N 個 opcode 時查的長度表，51 個 word（索引 0 是 opcode 0，長度 0）
	offAttackDivisor = 0x1012 // sub_18DAA
	offLevelDivisor  = 0x101A
	offClassBits     = 0x1022
	offToHit         = 0x103A // 命中門檻表，sub_8398 用
	offMultipliers   = 0x4DB8 // 怪物記錄的生命／經驗倍率（1,10,100,1000）
	offTerrainClass  = 0x52B2 // 野外地形碼的 32 項分類表，sub_5F40 用
	offExpTable      = 0x2E5C // 升級經驗表，sub_CC8C 用；每組 stride 0x24，索引 0 是等級 0
	offThresholds    = 0x10EA // sub_19A3C
	offBands         = 0x10F6
	offSpecialPtr    = 0x10AA // 2COMBAT.img 0x80bb
	offSpecialFlagA  = 0x13F6 // 2COMBAT.img 0xb70c
	offSpecialFlagB  = 0x1416
	offSpecialEffect = 0x1436

	// 標籤在 DGROUP 的起點。每一組都是連續的 NUL 結尾字串。
	offClassNames     = 0x003E
	offRaceNames      = 0x007B
	offAlignmentNames = 0x0097
	offSexNames       = 0x00A9
	offConditionNames = 0x015A
	offBonusNames     = 0x4318 // 物品加成的屬性清單，沒有耐力

	opcodeCount  = 51
	classCount   = 8
	bandRows     = 4
	bandCols     = 4
	specialCount = 30
)

func main() {
	exePath := flag.String("exe", "workplace/orig/MM2/MM2.EXE", "原版 MM2.EXE")
	outDir := flag.String("out", gamedata.Dir(), "輸出目錄")
	flag.Parse()

	exe, err := os.ReadFile(*exePath)
	if err != nil {
		log.Fatal(err)
	}
	if len(exe) < exetext.MinSize {
		log.Fatalf("%s 只有 %d bytes，沒有尾部資料區；這不是完整的 MM2.EXE",
			*exePath, len(exe))
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	r := reader{exe}
	files := map[string]any{
		"opcodes.json":   r.opcodes(),
		"combat.json":    r.combat(),
		"encounter.json": r.encounter(),
		"specials.json":  r.specials(),
		"labels.json":    r.labels(),
		"experience.json": r.experience(),
		"terrain.json": gamedata.Terrain{
			Source: fmt.Sprintf("MM2.EXE DGROUP ds:%04X，32 項（sub_5F40 先把碼 & 0x1F）", offTerrainClass),
			Class:  r.bytes(offTerrainClass, 32),
		},
	}
	for name, v := range files {
		p := filepath.Join(*outDir, name)
		b, err := json.MarshalIndent(v, "", " ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Println("寫出", p)
	}

	// 產完立刻回讀一次。缺欄位或偏移抓錯，在這裡就會被擋下來，
	// 不會等到遊戲跑起來才出現一堆零。
	if _, err := gamedata.Load(*outDir); err != nil {
		log.Fatalf("回讀失敗：%v", err)
	}
	fmt.Println("回讀通過")
}

type reader struct{ exe []byte }

// byteAt 讀 DGROUP 偏移的一個位元組。
func (r reader) byteAt(off int) byte { return r.exe[exetext.DGroupBase+off] }

// wordAt 讀 DGROUP 偏移的一個 little-endian word。
func (r reader) wordAt(off int) int {
	i := exetext.DGroupBase + off
	return int(r.exe[i]) | int(r.exe[i+1])<<8
}

func (r reader) bytes(off, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = int(r.byteAt(off + i))
	}
	return out
}

func (r reader) words(off, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = r.wordAt(off + i*2)
	}
	return out
}

func (r reader) opcodes() gamedata.Opcodes {
	out := make([]int, opcodeCount)
	for i := range out {
		out[i] = r.wordAt(offOpLen + i*2)
	}
	return gamedata.Opcodes{
		Source:  fmt.Sprintf("MM2.EXE DGROUP ds:%04X，%d 個 word", offOpLen, opcodeCount),
		Lengths: out,
	}
}

func (r reader) combat() gamedata.Combat {
	return gamedata.Combat{
		Source: fmt.Sprintf("MM2.EXE DGROUP ds:%04X／%04X／%04X",
			offAttackDivisor, offLevelDivisor, offClassBits),
		AttackDivisor: r.bytes(offAttackDivisor, classCount),
		LevelDivisor:  r.bytes(offLevelDivisor, classCount),
		ClassBits:     r.bytes(offClassBits, classCount),
		ToHitThresholds: r.bytes(offToHit, 16),
		Multipliers:     r.words(offMultipliers, 4),
	}
}

func (r reader) encounter() gamedata.Encounter {
	bands := make([][]int, bandRows)
	for i := range bands {
		bands[i] = r.bytes(offBands+i*bandCols, bandCols)
	}
	return gamedata.Encounter{
		Source: fmt.Sprintf("MM2.EXE DGROUP ds:%04X（門檻）與 ds:%04X（分段）",
			offThresholds, offBands),
		Thresholds: r.bytes(offThresholds, 7),
		Bands:      bands,
	}
}

func (r reader) specials() []gamedata.SpecialAttack {
	out := make([]gamedata.SpecialAttack, specialCount)
	for i := range out {
		text, err := exetext.At(r.exe, r.wordAt(offSpecialPtr+i*2))
		if err != nil {
			log.Fatalf("第 %d 種特殊攻擊的字串讀不到：%v", i, err)
		}
		out[i] = gamedata.SpecialAttack{
			Index:    i,
			Announce: text,
			Effect:   gamedata.SpecialEffect(r.byteAt(offSpecialEffect + i)),
			FlagA:    r.byteAt(offSpecialFlagA + i),
			FlagB:    r.byteAt(offSpecialFlagB + i),
		}
	}
	return out
}

// labels 讀出介面上那幾組固定名稱。每一組都是從指定偏移開始、
// 連續 N 條 NUL 結尾的字串。
func (r reader) labels() gamedata.Labels {
	return gamedata.Labels{
		Source:     "MM2.EXE 尾部的 DGROUP 初值段",
		Classes:    r.labelRun(offClassNames, 8),
		Races:      r.labelRun(offRaceNames, 5),
		Alignments: r.labelRun(offAlignmentNames, 3),
		Sexes:      r.labelRun(offSexNames, 2),
		Conditions: r.labelRun(offConditionNames, 11),
		Bonuses:    r.labelRun(offBonusNames, 6),
	}
}

func (r reader) labelRun(off, n int) []gamedata.Label {
	out := make([]gamedata.Label, n)
	for i := range out {
		text, err := exetext.At(r.exe, off)
		if err != nil {
			log.Fatalf("讀 ds:%04X 的標籤：%v", off, err)
		}
		out[i] = gamedata.Label{Key: fmt.Sprintf("exe.%04X", off), Text: text}
		off += len(text) + 1
	}
	return out
}

// experience 讀出升級經驗表。
//
// `sub_CC8C` 算的是 `表[0x24 × 組 + min(等級,10) × 4]`，所以等級 2 的項
// 在表首 +8。11 級以上的分段遞增寫死在程式碼裡，照抄。
func (r reader) experience() gamedata.Experience {
	read := func(group int) []int {
		out := make([]int, 9) // 等級 2–10
		for i := range out {
			off := offExpTable + group*0x24 + (i+2)*4
			out[i] = r.wordAt(off) | r.wordAt(off+2)<<16
		}
		return out
	}
	return gamedata.Experience{
		Source:      "MM2.EXE DGROUP ds:2E5C（表）＋ 2MISC2.img sub_CC8C（11 級以上的分段）",
		Fast:        read(0),
		Slow:        read(1),
		SlowClasses: []int{1, 2, 4, 6}, // 遊俠、弓箭手、巫師、忍者
		Tiers: []gamedata.ExpTier{
			{From: 11, Max: 1, Step: 192000},
			{From: 12, Max: 1, Step: 192000},
			{From: 13, Max: 1, Step: 192000},
			{From: 14, Max: 1, Step: 384000},
			{From: 15, Max: 1, Step: 384000},
			{From: 16, Max: 5, Step: 768000},
			{From: 21, Max: 10, Step: 1536000},
			{From: 31, Max: 20, Step: 3072000},
			{From: 51, Max: 25, Step: 1638400},
			{From: 76, Max: 0, Step: 6144000},
		},
	}
}
