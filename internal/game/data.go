package game

import (
	"fmt"

	"github.com/wicanr2/mm2_cht/internal/gamedata"
	"github.com/wicanr2/mm2_cht/internal/i18n"
)

// 引擎用的資料由 internal/gamedata 從 data/*.json 載入。
//
// 這一層只做兩件事：把資料交給引擎，以及把資料型別接到 game 的名字上，
// 讓呼叫端不必同時 import 兩個套件。
//
// 數值不寫在 Go 原始碼裡 —— 原版的表是版權材料，玩家自備原版用
// `cmd/mm2data` 產生；手冊轉錄的部分（職業成長、法術）入版控。

// 資料型別的別名。
type (
	Spell         = gamedata.Spell
	SpellSchool   = gamedata.SpellSchool
	SpecialAttack = gamedata.SpecialAttack
	SpecialEffect = gamedata.SpecialEffect
)

const (
	SchoolCleric   = gamedata.SchoolCleric
	SchoolSorcerer = gamedata.SchoolSorcerer

	EffectNone      = gamedata.EffectNone
	EffectMind      = gamedata.EffectMind
	EffectFire      = gamedata.EffectFire
	EffectLightning = gamedata.EffectLightning
	EffectCold      = gamedata.EffectCold
	EffectEnergy    = gamedata.EffectEnergy
	EffectPoison    = gamedata.EffectPoison
	EffectAcid      = gamedata.EffectAcid
	EffectFrenzy    = gamedata.EffectFrenzy
)

// data 是目前這一份遊戲資料。由 UseData 設定。
//
// 用套件層的變數而不是逐層傳參，是因為資料在一次執行裡不會換 ——
// 換了就是換一套遊戲。代價是忘了載入會拿到零值，所以 Loaded 要能問，
// 而 NewWorld 與 NewSession 都會先問。
var data *gamedata.Data

// UseData 設定引擎要用的資料。
func UseData(d *gamedata.Data) { data = d }

// Loaded 回報資料載入了沒有。
func Loaded() bool { return data != nil }

// Data 回傳目前這一份資料，沒載入回 nil。
func Data() *gamedata.Data { return data }

// OpLen 回傳事件腳本 opcode 的長度（含 opcode 本身），未知回 0。
func OpLen(op byte) int {
	if data == nil {
		return 0
	}
	return data.OpLen(op)
}

// ShopGoods 回傳某一類商店在某座城賣的六件貨。
func ShopGoods(group, town int) (ids, extra []int) {
	if data == nil {
		return nil, nil
	}
	return data.ShopGoods(group, town)
}

// Spells 回傳全部法術。
func Spells() []Spell {
	if data == nil {
		return nil
	}
	out := make([]Spell, len(data.Spells))
	copy(out, data.Spells)
	return out
}

// SpecialAttacks 回傳全部特殊攻擊，順序即原版索引。
func SpecialAttacks() []SpecialAttack {
	if data == nil {
		return nil
	}
	out := make([]SpecialAttack, len(data.Specials))
	copy(out, data.Specials)
	return out
}

// Special 回傳第 i 種特殊攻擊。
func Special(i int) (SpecialAttack, bool) {
	if data == nil {
		return SpecialAttack{}, false
	}
	return data.Special(i)
}

// SpecialsByEffect 回傳指定效果的所有特殊攻擊。
func SpecialsByEffect(e SpecialEffect) []SpecialAttack {
	if data == nil {
		return nil
	}
	return data.SpecialsByEffect(e)
}

// AnnounceSpecial 組出播報句，格式與原版一致：怪物名、空格、播報字串。
// names 給了譯文就用譯文，沒有就用原文。
func AnnounceSpecial(monster string, s SpecialAttack, names map[string]string) string {
	if t, ok := names[s.Announce]; ok && t != "" {
		return monster + t
	}
	return monster + " " + s.Announce
}

// AttacksPerRound 回傳這個角色每回合的揮擊次數。
//
// 原版（`2COMBAT.img` `0x8EC5` 那兩行）：`等級 / 揮擊除數[職業] + 1`，
// 等級取自記錄的 `+0x71`（= +113，等級的第二份）。
//
// 除數用的是 `ds:101A` 那張表，不是相鄰的 `ds:1012` —— 後者算的是命中
// 擲骰的上限。兩張表緊鄰、形狀相似，很容易對調。
func (c *Character) AttacksPerRound() int {
	d := 1
	if data != nil {
		d = data.SwingDivisorFor(int(c.Class))
	}
	if n := c.EffectiveLevel()/d + 1; n >= 1 {
		return n
	}
	return 1
}

// RollEncounterBand 依原版的門檻表決定遭遇的怪物類別。
func RollEncounterBand(r *Rand) int {
	if data == nil {
		return 0
	}
	roll := r.Range(1, 100)
	for i, t := range data.Encounter.Thresholds {
		if roll <= t {
			return i
		}
	}
	return len(data.Encounter.Thresholds) - 1
}

// RollMonsterIndex 依原版的方式挑一隻怪物：類別決定基礎編號，
// 難度等級決定範圍。
func RollMonsterIndex(r *Rand, difficulty int) int {
	if data == nil {
		return 0
	}
	bands := data.Encounter.Bands
	row := RollEncounterBand(r)
	if row >= len(bands) {
		row = len(bands) - 1
	}
	band := bands[row]
	if len(band) < 2 {
		return 0
	}
	col := difficulty
	if col < 1 {
		col = 1
	}
	if col > len(band)-1 {
		col = len(band) - 1
	}
	span := band[col]
	if span < 1 {
		span = 1
	}
	if idx := band[0] + r.Range(1, span); idx <= 255 {
		return idx
	}
	return 255
}

// SpellsOf 回傳某一系某一級的法術。
func SpellsOf(school SpellSchool, level int) []Spell {
	if data == nil {
		return nil
	}
	var out []Spell
	for _, s := range data.Spells {
		if s.School == school && s.Level == level {
			out = append(out, s)
		}
	}
	return out
}

// EnsureData 在還沒載入時，從預設資料目錄載入一次。
//
// 呼叫端不必自己記得載入；忘了載入會拿到「缺少 data/xxx.json」這種
// 講得清楚的錯誤，而不是一整套零值算出來的怪結果。
func EnsureData() error {
	if data != nil {
		return nil
	}
	d, err := gamedata.Load(gamedata.Dir())
	if err != nil {
		return err
	}
	data = d
	// 譯文沒設定就載預設的那份。找不到檔案時 i18n 回一份空的 Catalog，
	// 顯示原文，遊戲照樣跑。
	if text == nil {
		if c, err := i18n.Load(i18n.DefaultPath); err == nil {
			text = c
		}
	}
	return nil
}

// text 是顯示用的譯文。沒設定就顯示原文。
var text interface{ Or(key, fallback string) string }

// UseText 設定顯示用的譯文（`internal/i18n` 的 Catalog）。
//
// 名稱本身放在 data/labels.json（原文，讀自 MM2.EXE），譯文放在
// translations/zh-Hant.json，兩邊靠 `exe.XXXX` 這個 key 對上。
// 原文與譯文都不進 Go 原始碼。
func UseText(c interface{ Or(key, fallback string) string }) { text = c }

// label 把一組標籤的第 i 項翻成顯示字串。
func label(list []gamedata.Label, i int, fallback string) string {
	l := gamedata.LabelAt(list, i)
	if l.Text == "" {
		return fallback
	}
	if text == nil {
		return l.Text
	}
	return text.Or(l.Key, l.Text)
}

// ClassName、RaceName、AlignName、SexName 是介面上顯示的名稱。
func ClassName(i int) string {
	if data == nil {
		return fmt.Sprintf("職業 %d", i)
	}
	return label(data.Labels.Classes, i, fmt.Sprintf("職業 %d", i))
}

func RaceName(i int) string {
	if data == nil {
		return fmt.Sprintf("種族 %d", i)
	}
	return label(data.Labels.Races, i, fmt.Sprintf("種族 %d", i))
}

func AlignName(i int) string {
	if data == nil {
		return fmt.Sprintf("陣營 %d", i)
	}
	return label(data.Labels.Alignments, i, fmt.Sprintf("陣營 %d", i))
}

func SexName(i int) string {
	if data == nil {
		return fmt.Sprintf("性別 %d", i)
	}
	return label(data.Labels.Sexes, i, fmt.Sprintf("性別 %d", i))
}

// AttackDivisorFor、SwingDivisorFor 是職業的兩張除數表（`ds:1012`／`ds:101A`）。
// 前者算命中上限、後者算揮擊次數 —— 兩張形狀相似又緊鄰，很容易對調
// （見 `docs/formats/08-combat.md`）。
func AttackDivisorFor(class int) int {
	if data == nil {
		return 1
	}
	return data.AttackDivisorFor(class)
}

func SwingDivisorFor(class int) int {
	if data == nil {
		return 1
	}
	return data.SwingDivisorFor(class)
}
