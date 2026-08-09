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
	// AttackDivisor 是命中上限的除數（ds:1012）：命中擲骰的上限是
	// `25 + 等級 / 這個除數`。
	AttackDivisor []int `json:"attackDivisor"`
	// LevelDivisor 是每回合揮擊次數的除數（ds:101A）：
	// 揮擊次數 = `等級 / 這個除數 + 1`。
	//
	// 兩者容易對調 —— `2COMBAT.img` 的 `0x8EC5` 那兩行緊鄰，
	// 上面那個算的是命中上限，下面那個才是揮擊次數。
	LevelDivisor []int `json:"levelDivisor"`
	// ClassBits 是職業的位元遮罩（ds:1022）。
	ClassBits []int `json:"classBits"`
	// ToHitThresholds 是命中門檻（ds:103A），依攻擊者的怪物編號高 nibble 索引。
	// 命中率（百分比）= 門檻 − 目標的防護等級，目標防護超過門檻時固定 5。
	ToHitThresholds []int `json:"toHitThresholds"`
	// MagicResist 是怪物的抗魔法百分比，
	// 用怪物記錄 `+17 >> 5` 索引（原版 `ds:4DC0`）。
	MagicResist []int `json:"magicResist"`

	// FlightMaps 是飛行術的目的地表：5 欄（A–E）× 4 列，
	// 索引 `欄 * 4 + 列`。值全是野外地圖編號（5–16、33–40）。
	FlightMaps []int `json:"flightMaps"`

	// ShopStock 是商店貨架，從 `ds:43C8` 起連續八張 30 bytes 的表，
	// 兩張一組共四組。每組的第一張是物品編號、第二張是附屬值
	// （附魔等級或充能），各 5 城 × 6 件。
	ShopStock []int `json:"shopStock"`

	// 自然之門的三張表：日期門檻（13 個）、地圖與座標（各 14 項）。
	// 索引由當天的日子挑出來，見 `docs/formats/09-spells.md`。
	GateDays []int `json:"gateDays"`
	GateMaps []int `json:"gateMaps"`
	GatePos  []int `json:"gatePos"`
	// StatBands 是屬性修正的門檻表（原版 `ds:4D84`，23 項）。
	// 修正值 = −3 加上「小於該屬性值的門檻個數」。
	StatBands []int `json:"statBands"`
	// Multipliers 是怪物記錄裡生命與經驗的倍率表（ds:4DB8）：1／10／100／1000。
	Multipliers []int `json:"multipliers"`
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

// ExpTier 是等級 10 以上的經驗遞增段。
//
// 級數 = clamp(等級 − From + 1, 0, Max)，`Max` 為 0 表示不設上限。
type ExpTier struct {
	From int `json:"from"`
	Max  int `json:"max"`
	Step int `json:"step"`
}

// Experience 是升級所需的累計經驗值。
//
// 出自 `2MISC2.img` 的 `sub_CC8C`：等級 2–10 直接查表（依職業分兩組），
// 11 級以上改成分段的等差累加。
type Experience struct {
	Source string `json:"source"`
	// Fast、Slow 是等級 2–10 的門檻，索引 0 對應等級 2。
	// Fast 是武士／牧師／賊／野蠻人，Slow 是遊俠／弓箭手／巫師／忍者。
	Fast []int `json:"fast"`
	Slow []int `json:"slow"`
	// SlowClasses 是走 Slow 那張表的職業編號。
	SlowClasses []int `json:"slowClasses"`
	// Tiers 是 11 級以上的遞增段。
	Tiers []ExpTier `json:"tiers"`
}

// Terrain 是野外地形碼的分類表。
type Terrain struct {
	Source string `json:"source"`
	// Class 是 32 項表：`地形碼 & 0x1F` → 類別 0–4。
	Class []int `json:"class"`
}

// 野外地形的類別。出自 `2PLAY.img` `sub_5E68` 的室外分支。
const (
	TerrainOpen     = 0 // 可通行
	TerrainMountain = 1 // 山區，需要隊伍裡有兩名登山家
	TerrainClass2   = 2 // 語意未定，野外圖上一格都沒有
	TerrainForest   = 3 // 森林，需要隊伍裡有兩名探險家
	TerrainWater    = 4 // 水域
)

// 穿越山區與森林要用的第二技能代碼。原版寫死在 `sub_5E68` 裡
// （`sub_36A6(0x0B)` 與 `sub_36A6(0x0D)`），與手冊的技能編號一致。
const (
	SkillMountaineer = 11 // 登山家
	SkillPathfinder  = 13 // 探險家
	// SkillPartyNeeded 是需要幾人具備才能通過。手冊：「隊伍中有二人或
	// 二人以上具有此技能時可穿越」——原版的判斷正是 `si < 2`。
	SkillPartyNeeded = 2
)

// Label 是一個可翻譯的標籤：原文加上譯文檔裡的 key。
//
// key 是 `exe.XXXX`（XXXX 是 DGROUP 偏移），與 `cmd/mm2strings` 匯出時一致。
// 顯示端拿 key 去 i18n 查譯文，查不到就顯示 Text。
type Label struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// Fields 是事件腳本用來讀寫角色欄位的選擇器表。
//
// 出自 `2PLAY.OVL` 的 `sub_1AA00`：128 項的跳表，每一項把
// 「角色記錄基底 + N」寫進 `ds:9FF2`，寬度寫進 `ds:9FF1`。
// 這是原版自己列出來的角色記錄欄位圖，不是從資料猜的。
type Fields struct {
	Source string  `json:"source"`
	Sel    []Field `json:"sel"`
}

// Field 是一個選擇器：記錄裡的偏移與寬度（1／2／4 bytes）。
// Offset 為 -1 表示那一項不是單純的「基底 + 位移」，語意未解。
type Field struct {
	Offset int `json:"offset"`
	Width  int `json:"width"`
}

// Lookup 回傳選擇器對應的欄位；未解或超出範圍時 ok 為 false。
func (f Fields) Lookup(sel int) (Field, bool) {
	if sel < 0 || sel >= len(f.Sel) || f.Sel[sel].Offset < 0 {
		return Field{}, false
	}
	return f.Sel[sel], true
}

// Traps 是開鎖失敗時觸發的陷阱。
//
// 出自 `2MISC.OVL`：`sub_1C41E` 用 `(場景 × 16 + 種類 × 4)` 去 `ds:28F2`
// 取訊息指標，`sub_1C338` 用 `基礎[場景] << ATTRIB+20` 算傷害。
// 四種種類依訊息內容是電擊／火焰／毒氣／尖刺。
type Traps struct {
	Source string `json:"source"`
	// Base 是五種場景的基礎傷害（`ds:2946`）。
	Base []int `json:"base"`
	// Text[場景][種類] 是原版的播報文字。
	Text [][]Label `json:"text"`
	// Announce 是觸發時先印的那一句（`ds:2950`）。
	Announce Label `json:"announce"`
}

// TrapKind 是陷阱的種類，依原版訊息內容命名。
const (
	TrapShock = 0 // 電擊
	TrapFire  = 1 // 火焰
	TrapGas   = 2 // 毒氣
	TrapSpike = 3 // 尖刺／金屬
)

// TrapScene 把地圖的場景碼換成陷阱表的索引（原版 `sub_1CA00`）。
func TrapScene(code int) int {
	switch code {
	case 0:
		return 0
	case 3:
		return 1
	case 1:
		return 2
	case 4, 6:
		return 3
	}
	return 4
}

// TrapDamage 回傳某個場景、某個位移量下的陷阱傷害。
func (t Traps) Damage(scene, shift int) int {
	if scene < 0 || scene >= len(t.Base) {
		return 0
	}
	return t.Base[scene] << shift
}

// TrapText 回傳陷阱的播報文字。
func (t Traps) TrapText(scene, kind int) Label {
	if scene < 0 || scene >= len(t.Text) || kind < 0 || kind >= len(t.Text[scene]) {
		return Label{}
	}
	return t.Text[scene][kind]
}

// Pictures 是事件腳本 opcode `0x0b` 的圖號對照。
//
// 出自 `sub_18EE6`：參數 ≥ 0x80 一律是 75；否則參數減一，再依場景碼
// `ds:039C`（0–6）挑一張表去查。表裡的值是 `monsters.16` 的圖號
// （1–75），高號那幾張正是房舍、城堡、店家、寶箱。
type Pictures struct {
	Source string  `json:"source"`
	Tables [][]int `json:"tables"`
	// Scene 把場景碼（0–6）對到 Tables 的第幾張。原版是一張七項跳表，
	// 場景 2 與 5 共用一張、4 與 6 共用一張。
	Scene []int `json:"scene"`
}

// Picture 回傳 `0x0b` 參數在某個場景下的圖號，0 表示查不到。
func (p Pictures) Picture(scene, arg int) int {
	if arg >= 0x80 {
		return 75
	}
	if scene < 0 || scene >= len(p.Scene) {
		return 0
	}
	t := p.Scene[scene]
	if t < 0 || t >= len(p.Tables) {
		return 0
	}
	i := arg - 1
	if i < 0 || i >= len(p.Tables[t]) {
		return 0
	}
	return p.Tables[t][i]
}

// SpellCosts 是 `SPELLS.DAT` 解出來的施法代價，順序與 `spells.json` 相同。
//
// 檔案本身是 96 × 2 bytes，**前 48 筆是巫師、後 48 筆是牧師** ——
// root 的 `sub_15644` 寫得很清楚：職業不是 2（弓箭手）也不是 4（巫師）
// 就把序號加 0x30（48）再去 `ds:7D60` 查。`spells.json` 的順序相反
// （前 48 是牧師），所以產生時已經對調過。
//
// 兩個位元組的欄位：
//
//	A 低 nibble  寶石消耗（超過 15 的另有編碼，未解）
//	A 高 nibble  旗標，未解
//	B 位元 0–3   固定法力消耗
//	B 位元 4–6   每施法者等級的法力消耗
//	B 位元 7     旗標，未解（12 條有，含各種傳送與飛行）
//
// 與手冊逐條比對：可解析的 92 條裡固定與每等級各 84 條相符、寶石 87 條相符。
// 對不上的是手冊自己標錯「每等級」的八條，與寶石超過 15 的五條。
// **裁決以程式為準**（CLAUDE.md 的 oracle 順序）。
type SpellCosts struct {
	Source string      `json:"source"`
	Entry  []SpellCost `json:"entry"`
}

// SpellCost 是一條法術的原始兩個位元組。
type SpellCost struct {
	A byte `json:"a"`
	B byte `json:"b"`
}

// Gems 是寶石消耗。
func (c SpellCost) Gems() int { return int(c.A & 0x0F) }

// SP 是施法者等級為 level 時的法力消耗：固定值加上每等級的量。
func (c SpellCost) SP(level int) int {
	return int(c.B&0x0F) + int((c.B>>4)&7)*level
}

// SpellCostAt 取第 i 條法術的代價，超出範圍回零值。
func (d *Data) SpellCostAt(i int) SpellCost {
	if i < 0 || i >= len(d.SpellCosts.Entry) {
		return SpellCost{}
	}
	return d.SpellCosts.Entry[i]
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
	Classes    Classes
	Terrain    Terrain
	Experience Experience
	Labels     Labels
	Fields     Fields
	Traps      Traps
	Pictures   Pictures
	SpellCosts SpellCosts
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
		{"experience.json", &d.Experience, true},
		{"terrain.json", &d.Terrain, true},
		{"fields.json", &d.Fields, true},
		{"traps.json", &d.Traps, true},
		{"pictures.json", &d.Pictures, true},
		{"spellcosts.json", &d.SpellCosts, true},
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
	case len(d.Combat.ToHitThresholds) != 16:
		return fmt.Errorf("combat.json 的 toHitThresholds 應該有 16 項，實際 %d",
			len(d.Combat.ToHitThresholds))
	case len(d.Combat.AttackDivisor) != 8:
		return fmt.Errorf("combat.json 的 attackDivisor 應該有 8 項，實際 %d",
			len(d.Combat.AttackDivisor))
	case len(d.SpellCosts.Entry) != 96:
		return fmt.Errorf("spellcosts.json 應該有 96 條，實際 %d", len(d.SpellCosts.Entry))
	case len(d.Pictures.Tables) != 5 || len(d.Pictures.Scene) != 7:
		return fmt.Errorf("pictures.json 應該有 5 張表與 7 個場景，實際 %d／%d",
			len(d.Pictures.Tables), len(d.Pictures.Scene))
	case len(d.Traps.Base) != 5 || len(d.Traps.Text) != 5:
		return fmt.Errorf("traps.json 應該有 5 種場景，實際 %d／%d",
			len(d.Traps.Base), len(d.Traps.Text))
	case len(d.Fields.Sel) != 128:
		return fmt.Errorf("fields.json 應該有 128 個選擇器，實際 %d", len(d.Fields.Sel))
	case len(d.Encounter.Thresholds) == 0:
		return fmt.Errorf("encounter.json 沒有門檻表")
	case len(d.Encounter.Bands) == 0:
		return fmt.Errorf("encounter.json 沒有分段表")
	case len(d.Classes.HitDice) != 8:
		return fmt.Errorf("classes.json 的 hitDice 應該有 8 項，實際 %d", len(d.Classes.HitDice))
	case len(d.Specials) == 0:
		return fmt.Errorf("specials.json 是空的")
	case len(d.Experience.Fast) != 9 || len(d.Experience.Slow) != 9:
		return fmt.Errorf("experience.json 的兩張表都應該有 9 項（等級 2–10）")
	case len(d.Terrain.Class) != 32:
		return fmt.Errorf("terrain.json 的 class 應該有 32 項，實際 %d", len(d.Terrain.Class))
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

// ToHitPercent 回傳命中率（百分比，1–100）。
//
// 出自 `2COMBAT.img` 的 `sub_8398`：門檻由攻擊者的怪物編號高 nibble 查
// `ds:103A`，減掉目標的防護等級。目標防護等級高於門檻時保底 5%。
// StatBonus 回傳一個屬性值的修正。
//
// 抄自 `sub_1354A`：從 −3 起，門檻表裡每有一項小於這個值就加一。
// 表是 `ds:4D84` = 4,6,9,13,15,17,19,22,26,30,45,60,…,250,255，
// 所以 10–13 是 0、14–15 是 +1、超過 250 是 +19。
func (d *Data) StatBonus(v int) int {
	b := -3
	for _, t := range d.Combat.StatBands {
		if t >= v {
			break
		}
		b++
	}
	return b
}

// NatureGate 依日子算出自然之門的落點（地圖與 X／Y）。
//
// 只有**單數日**才查門檻表：從頭找第一個大於當天日子的門檻，
// 那一格的索引就是答案；找不到就用預設的 12。日子到 150 以上
// 改用索引 11。偶數日一律用預設的 12。
func (d *Data) NatureGate(day int) (m, x, y int, ok bool) {
	idx := 12
	if day%2 != 0 {
		for i, th := range d.Combat.GateDays {
			if th > day {
				idx = i
				break
			}
		}
	}
	if day >= 150 {
		idx = 11
	}
	if idx >= len(d.Combat.GateMaps) || idx >= len(d.Combat.GatePos) {
		return 0, 0, 0, false
	}
	p := d.Combat.GatePos[idx]
	return d.Combat.GateMaps[idx], p & 0x0F, p >> 4, true
}

// ShopGoods 回傳某一類商店在某座城賣的六件貨（編號與附屬值）。
//
// group 0–3 選商店類別，town 0–4 選城（原版用 `ds:0392` 也就是
// 地圖編號直接乘 6，所以城就是地圖 0–4）。
func (d *Data) ShopGoods(group, town int) (ids, extra []int) {
	if group < 0 || group > 3 || town < 0 || town > 4 {
		return nil, nil
	}
	base := group * 60
	i := base + town*6
	j := base + 30 + town*6
	if j+6 > len(d.Combat.ShopStock) {
		return nil, nil
	}
	return d.Combat.ShopStock[i : i+6], d.Combat.ShopStock[j : j+6]
}

// FlightMap 回傳飛行術某一格的地圖編號，格子不存在回 -1。
func (d *Data) FlightMap(col, row int) int {
	i := col*4 + row
	if col < 0 || col > 4 || row < 0 || row > 3 || i >= len(d.Combat.FlightMaps) {
		return -1
	}
	return d.Combat.FlightMaps[i]
}

// MonsterMagicResist 回傳怪物的抗魔法百分比，用記錄 `+17 >> 5` 索引。
func (d *Data) MonsterMagicResist(index int) int {
	if index < 0 || index >= len(d.Combat.MagicResist) {
		return 100
	}
	return d.Combat.MagicResist[index]
}

func (d *Data) ToHitPercent(attackerTier, targetAC int) int {
	th := d.Combat.ToHitThresholds
	if attackerTier < 0 || attackerTier >= len(th) {
		return 5
	}
	if targetAC > th[attackerTier] {
		return 5
	}
	return th[attackerTier] - targetAC
}

// AttackDivisorFor 回傳命中上限的除數，至少 1。
func (d *Data) AttackDivisorFor(class int) int {
	return divisor(d.Combat.AttackDivisor, class)
}

// SwingDivisorFor 回傳揮擊次數的除數，至少 1。
func (d *Data) SwingDivisorFor(class int) int {
	return divisor(d.Combat.LevelDivisor, class)
}

func divisor(t []int, class int) int {
	if class < 0 || class >= len(t) || t[class] < 1 {
		return 1
	}
	return t[class]
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
// 抄自 `2MISC2.img` 的 `sub_CC8C`。等級 2–10 直接查表，11 級以上把分段的
// 等差一段一段加上去；查表的索引在 10 封頂，所以高等級是「表的最後一項
// 加上各段累積」，不是繼續倍增。
func (d *Data) ExpForLevel(level, class int) int {
	if level <= 1 {
		return 0
	}
	e := d.Experience
	table := e.Fast
	for _, c := range e.SlowClasses {
		if c == class {
			table = e.Slow
			break
		}
	}
	if len(table) == 0 {
		return 0
	}
	i := level - 2
	if i >= len(table) {
		i = len(table) - 1
	}
	req := table[i]
	for _, t := range e.Tiers {
		n := level - t.From + 1
		if n <= 0 {
			continue
		}
		if t.Max > 0 && n > t.Max {
			n = t.Max
		}
		req += n * t.Step
	}
	return req
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

// TerrainClass 回傳野外地形碼的類別。
func (d *Data) TerrainClass(code byte) int {
	i := int(code & 0x1F)
	if i >= len(d.Terrain.Class) {
		return TerrainOpen
	}
	return d.Terrain.Class[i]
}
