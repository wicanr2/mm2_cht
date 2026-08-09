// Package gamedata 載入遊戲資料。
//
// 引擎（internal/game 以下）只放規則與流程，**數值一律從 data/*.json 讀**。
// 分開的理由有三個：
//
//   - 原版的表是版權材料，不入版控。玩家自備原版檔，用 `cmd/mm2data`
//     從自己的 MM2.EXE 產生，產物在 .gitignore 裡。
//   - 數值改了不必重編。對照原版調整時省掉整個 build 循環。
//   - 「這個值哪來的」看得見。JSON 帶 source 欄位寫明出處，
//     翻進 Go 常數就只剩一串數字。
//
// 資料目錄預設是 `data/`，環境變數 `MM2_DATA_DIR` 可以覆寫。
package gamedata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DirEnv 是覆寫資料目錄的環境變數。
const DirEnv = "MM2_DATA_DIR"

// Dir 回傳要用的資料目錄。
func Dir() string {
	if d := os.Getenv(DirEnv); d != "" {
		return d
	}
	return "data"
}

// SpellSchool 是法術體系。
type SpellSchool byte

const (
	SchoolCleric SpellSchool = iota
	SchoolSorcerer
)

func (s SpellSchool) String() string {
	if s == SchoolSorcerer {
		return "巫師"
	}
	return "牧師"
}

// Spell 是一條法術。資料抄自手冊，不是從 overlay 解出來的。
type Spell struct {
	School SpellSchool
	Level  int    // 1–9
	Index  int    // 該級之內的序號，1 起算
	Name   string // 官方中文名
	Origin string // 英文原名
	Cost   string // 消耗，照手冊記法
	Form   string // 形式：戰鬥用／非戰鬥用／任何時刻…
	Target string // 對象
	Desc   string // 說明
}

// SpecialEffect 是特殊攻擊的效果編號（`ds:1436` 的原始值）。
//
// 同編號的攻擊在元素上完全同群 —— 火系三種都是 23、電系三種都是 24 ——
// 所以它識別的是傷害元素或對應的視覺效果。分群是資料事實，
// 「編號代表元素」是由分群推得的解釋（強推論）。
type SpecialEffect byte

const (
	EffectNone      SpecialEffect = 0
	EffectMind      SpecialEffect = 21 // 凝視、催眠
	EffectFire      SpecialEffect = 23
	EffectLightning SpecialEffect = 24
	EffectCold      SpecialEffect = 25
	EffectEnergy    SpecialEffect = 26
	EffectPoison    SpecialEffect = 28 // 毒與毒氣
	EffectAcid      SpecialEffect = 29
	EffectFrenzy    SpecialEffect = 31
)

var effectNames = map[SpecialEffect]string{
	EffectNone: "無元素", EffectMind: "精神", EffectFire: "火", EffectLightning: "電",
	EffectCold: "寒冰", EffectEnergy: "能量", EffectPoison: "毒", EffectAcid: "酸",
	EffectFrenzy: "狂亂",
}

func (e SpecialEffect) String() string {
	if s, ok := effectNames[e]; ok {
		return s
	}
	return fmt.Sprintf("效果 %d", byte(e))
}

// SpecialAttack 是怪物的一種特殊攻擊。
type SpecialAttack struct {
	Index int
	// Announce 是原版的播報字串，接在怪物名後面。
	Announce string
	Effect   SpecialEffect
	// FlagA、FlagB 是 ds:13F6 與 ds:1416 的值，語意未定。
	// 值 99 表示這一項不走共用路徑，由別處處理。
	FlagA, FlagB byte
}

// Handled 回報這一項是否走 `2COMBAT.img` 0xb70c 那條共用路徑。
func (s SpecialAttack) Handled() bool { return s.FlagA != 99 }

// Opcodes 是事件腳本的 opcode 長度表。
type Opcodes struct {
	Source  string `json:"source"`
	Lengths []int  `json:"lengths"`
}

// Combat 是戰鬥用的職業表。
type Combat struct {
	Source string `json:"source"`
	// AttackDivisor 是算每回合攻擊次數的除數，依職業索引（ds:1012）。
	AttackDivisor []int `json:"attackDivisor"`
	// LevelDivisor 是同一段程式碼的第二個除數（ds:101A），用途待確認。
	LevelDivisor []int `json:"levelDivisor"`
	// ClassBits 是職業的位元遮罩（ds:1022）。
	ClassBits []int `json:"classBits"`
}

// Encounter 是遭遇用的表。
type Encounter struct {
	Source string `json:"source"`
	// Thresholds 是決定怪物類別的門檻（ds:10EA）。
	Thresholds []int `json:"thresholds"`
	// Bands 是每個類別的基礎編號與三個難度的範圍（ds:10F6 起，四個一組）。
	Bands [][]int `json:"bands"`
}

// Classes 是職業的成長參數，抄自手冊。
type Classes struct {
	Source string `json:"source"`
	// HitDice 是每升一級擲的生命點數上限，依職業索引。
	HitDice []int `json:"hitDice"`
	// SpellLevelAt 是「法力等級 N 需要的經驗等級」。
	SpellLevelAt []int `json:"spellLevelAt"`
	// ExpStep 是升級經驗表的等差係數。**暫定**，原版的表還沒定位。
	ExpStep int `json:"expStep"`
}

// Label 是一個可翻譯的標籤：原文加上譯文檔裡的 key。
//
// key 是 `exe.XXXX`（XXXX 是 DGROUP 偏移），與 `cmd/mm2strings` 匯出時一致。
// 顯示端拿 key 去 i18n 查譯文，查不到就顯示 Text。
type Label struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// Labels 是介面上會出現的幾組固定名稱，全部讀自 MM2.EXE 尾部。
//
// 這些名稱以前寫死在 Go 原始碼裡（`var classNames = [...]string{"武士", …}`），
// 那違反「翻譯文本不進原始碼」——原文與譯文都該在資料層。
type Labels struct {
	Source     string  `json:"source"`
	Classes    []Label `json:"classes"`
	Races      []Label `json:"races"`
	Alignments []Label `json:"alignments"`
	Sexes      []Label `json:"sexes"`
	Conditions []Label `json:"conditions"`
	// Bonuses 是物品加成會用到的屬性清單（力量／智慧／人格／速度／準確度／運氣）。
	// **不是人物的六項屬性** —— 這一組沒有耐力，順序也不同。
	Bonuses []Label `json:"bonuses"`
}

// Data 是一整份遊戲資料。
type Data struct {
	Opcodes   Opcodes
	Combat    Combat
	Encounter Encounter
	Classes   Classes
	Labels    Labels
	Specials  []SpecialAttack
	Spells    []Spell
}

// Load 從指定目錄讀入全部資料。
//
// 原版衍生的四個檔（opcodes / combat / encounter / specials）缺檔時回明確的
// 錯誤，指向 `cmd/mm2data`——那些不入版控，要玩家自己從原版產生。
func Load(dir string) (*Data, error) {
	d := &Data{}
	for _, f := range []struct {
		name string
		v    any
		gen  bool
	}{
		{"opcodes.json", &d.Opcodes, true},
		{"combat.json", &d.Combat, true},
		{"encounter.json", &d.Encounter, true},
		{"specials.json", &d.Specials, true},
		{"labels.json", &d.Labels, true},
		{"classes.json", &d.Classes, false},
		{"spells.json", &d.Spells, false},
	} {
		p := filepath.Join(dir, f.name)
		b, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) && f.gen {
				return nil, fmt.Errorf("缺少 %s；這個檔由原版 MM2.EXE 產生，"+
					"請先跑 `go run ./cmd/mm2data -exe <你的 MM2.EXE> -out %s`", p, dir)
			}
			return nil, err
		}
		if err := json.Unmarshal(b, f.v); err != nil {
			return nil, fmt.Errorf("解析 %s: %w", p, err)
		}
	}
	return d, d.validate()
}

// MustLoad 是 Load 的 panic 版，給 main 與測試用。
func MustLoad(dir string) *Data {
	d, err := Load(dir)
	if err != nil {
		panic(err)
	}
	return d
}

func (d *Data) validate() error {
	switch {
	case len(d.Opcodes.Lengths) == 0:
		return fmt.Errorf("opcodes.json 沒有長度表")
	case len(d.Combat.AttackDivisor) != 8:
		return fmt.Errorf("combat.json 的 attackDivisor 應該有 8 項，實際 %d",
			len(d.Combat.AttackDivisor))
	case len(d.Encounter.Thresholds) == 0:
		return fmt.Errorf("encounter.json 沒有門檻表")
	case len(d.Encounter.Bands) == 0:
		return fmt.Errorf("encounter.json 沒有分段表")
	case len(d.Classes.HitDice) != 8:
		return fmt.Errorf("classes.json 的 hitDice 應該有 8 項，實際 %d", len(d.Classes.HitDice))
	case len(d.Specials) == 0:
		return fmt.Errorf("specials.json 是空的")
	case len(d.Labels.Classes) != 8:
		return fmt.Errorf("labels.json 的 classes 應該有 8 項，實際 %d", len(d.Labels.Classes))
	case len(d.Spells) == 0:
		return fmt.Errorf("spells.json 是空的")
	}
	return nil
}

// OpLen 回傳 opcode 的長度（含 opcode 本身），未知回 0。
func (d *Data) OpLen(op byte) int {
	if int(op) >= len(d.Opcodes.Lengths) {
		return 0
	}
	return d.Opcodes.Lengths[op]
}

// AttackDivisorFor 回傳職業的攻擊次數除數，至少 1。
func (d *Data) AttackDivisorFor(class int) int {
	if class < 0 || class >= len(d.Combat.AttackDivisor) {
		return 1
	}
	if v := d.Combat.AttackDivisor[class]; v >= 1 {
		return v
	}
	return 1
}

// HitDice 回傳職業每級的生命點數上限。
func (d *Data) HitDice(class int) int {
	if class < 0 || class >= len(d.Classes.HitDice) {
		return 8
	}
	return d.Classes.HitDice[class]
}

// SpellLevelFor 回傳這個經驗等級對應的最高法力等級。
func (d *Data) SpellLevelFor(level int) int {
	lv := 0
	for i, need := range d.Classes.SpellLevelAt {
		if level >= need {
			lv = i + 1
		}
	}
	return lv
}

// ExpForLevel 是升到指定等級所需的累計經驗值。
//
// **暫定**：手冊沒給這張表，原版的表還沒在 DGROUP 影像裡定位。
// 這裡取每級遞增的等差，形狀合理（越後面越貴），數字不是原版的。
func (d *Data) ExpForLevel(level int) int {
	if level <= 1 {
		return 0
	}
	step := d.Classes.ExpStep
	if step <= 0 {
		step = 500
	}
	n := level - 1
	return step * n * (n + 1) / 2
}

// Special 回傳第 i 種特殊攻擊。
func (d *Data) Special(i int) (SpecialAttack, bool) {
	if i < 0 || i >= len(d.Specials) {
		return SpecialAttack{}, false
	}
	return d.Specials[i], true
}

// SpecialsByEffect 回傳指定效果的所有特殊攻擊。
func (d *Data) SpecialsByEffect(e SpecialEffect) []SpecialAttack {
	var out []SpecialAttack
	for _, s := range d.Specials {
		if s.Effect == e {
			out = append(out, s)
		}
	}
	return out
}

// LabelAt 從一組標籤裡取第 i 個，越界回空的 Label。
func LabelAt(list []Label, i int) Label {
	if i < 0 || i >= len(list) {
		return Label{}
	}
	return list[i]
}
