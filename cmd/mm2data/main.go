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
	offTrapBase   = 0x2946 // 五種場景的基礎傷害
	offTrapText   = 0x28F2 // 場景 × 16 + 種類 × 4 的訊息指標表
	offTrapPrompt = 0x2950 // 觸發時先印的那一句

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
	ovlPath := flag.String("play-ovl", "workplace/orig/MM2/2PLAY.OVL", "原版 2PLAY.OVL")
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
		"fields.json": readFields(*ovlPath),
		"traps.json":  r.traps(),
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

// traps 讀出開鎖陷阱的傷害與播報文字。
//
// 索引方式抄自 `sub_1C41E`：`di = 種類 × 4 + 場景 × 16`，
// 從 `ds:28F2` 取一個 word 當字串偏移。
func (r reader) traps() gamedata.Traps {
	text := make([][]gamedata.Label, 5)
	for scene := range text {
		text[scene] = make([]gamedata.Label, 4)
		for kind := range text[scene] {
			off := r.wordAt(offTrapText + kind*4 + scene*16)
			s, err := exetext.At(r.exe, off)
			if err != nil {
				log.Fatalf("陷阱訊息（場景 %d 種類 %d）：%v", scene, kind, err)
			}
			text[scene][kind] = gamedata.Label{Key: fmt.Sprintf("exe.%04X", off), Text: s}
		}
	}
	prompt, err := exetext.At(r.exe, offTrapPrompt)
	if err != nil {
		log.Fatal(err)
	}
	return gamedata.Traps{
		Source: fmt.Sprintf("MM2.EXE DGROUP ds:%04X（傷害）與 ds:%04X（訊息）",
			offTrapBase, offTrapText),
		Base:     r.words(offTrapBase, 5),
		Text:     text,
		Announce: gamedata.Label{Key: fmt.Sprintf("exe.%04X", offTrapPrompt), Text: prompt},
	}
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

// ── 事件腳本的角色欄位選擇器 ────────────────────────────────────────────

// 2PLAY.OVL 在重建的映像裡載入到偏移 0x7E10（整個檔逐位元組相符），
// 所以「映像偏移 − 0x7E10」就是 OVL 檔內偏移。
const (
	playOvlBase = 0x7E10
	// selTable 是 sub_1AA00 的 128 項跳表在映像裡的偏移。
	selTable = 0xAECE
	selCount = 128
)

// 寬度不是 1 的選擇器。sub_1AA00 一開始就用 cmp/je 把它們挑出來，
// 寫進 ds:9FF1（1 = byte、2 = word、4 = dword）。
var (
	selWidth4 = []int{0x31, 0x3E}
	selWidth2 = []int{0x20, 0x28, 0x35, 0x38, 0x3A, 0x3C}
)

// readFields 從 2PLAY.OVL 解出選擇器 → 角色記錄偏移的對照。
//
// 每一項的本體都是同一個形狀：
//
//	8B 46 04           mov ax, [bp+4]      ; 角色記錄基底
//	05 lo hi           add ax, N           ; （或 83 C0 nn，或 40 = inc）
//	EB/E9 rel          jmp 共用尾巴        ; mov ds:9FF2, ax
//
// 128 項裡 126 項是這個形狀。剩下兩項（0x00、0x01）呼叫 sub_1B0B2，
// 不是單純的「基底 + 位移」，標成 -1。
func readFields(path string) gamedata.Fields {
	ovl, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	at := func(imgOff int) int { return imgOff - playOvlBase }
	if at(selTable)+selCount*2 > len(ovl) {
		log.Fatalf("%s 只有 %d bytes，放不下選擇器跳表", path, len(ovl))
	}
	width := make([]int, selCount)
	for i := range width {
		width[i] = 1
	}
	for _, s := range selWidth2 {
		width[s] = 2
	}
	for _, s := range selWidth4 {
		width[s] = 4
	}

	out := gamedata.Fields{
		Source: fmt.Sprintf("2PLAY.OVL sub_1AA00 的 %d 項跳表（映像偏移 %#X）", selCount, selTable),
		Sel:    make([]gamedata.Field, selCount),
	}
	decoded := 0
	for i := 0; i < selCount; i++ {
		t := at(selTable) + i*2
		body := at(int(ovl[t]) | int(ovl[t+1])<<8)
		off, ok := fieldOffset(ovl, body)
		if !ok {
			out.Sel[i] = gamedata.Field{Offset: -1, Width: width[i]}
			continue
		}
		out.Sel[i] = gamedata.Field{Offset: off, Width: width[i]}
		decoded++
	}
	if decoded < 120 {
		log.Fatalf("選擇器只解出 %d 項，跳表位置或位移可能不對", decoded)
	}
	return out
}

func fieldOffset(ovl []byte, at int) (int, bool) {
	if at < 0 || at+6 > len(ovl) {
		return 0, false
	}
	b := ovl[at:]
	if b[0] != 0x8B || b[1] != 0x46 || b[2] != 0x04 { // mov ax,[bp+4]
		return 0, false
	}
	i, add := 3, 0
	switch {
	case b[3] == 0x05: // add ax, imm16
		add, i = int(b[4])|int(b[5])<<8, 6
	case b[3] == 0x83 && b[4] == 0xC0: // add ax, imm8
		add, i = int(b[5]), 6
	case b[3] == 0x40: // inc ax
		add, i = 1, 4
	}
	// 尾巴要嘛就地寫入，要嘛跳到共用的 mov ds:9FF2, ax。
	if b[i] == 0xEB || b[i] == 0xE9 ||
		(b[i] == 0xA3 && b[i+1] == 0xF2 && b[i+2] == 0x9F) {
		return add, true
	}
	return 0, false
}
