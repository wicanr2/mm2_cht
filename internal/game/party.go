package game

import (
	"bytes"
	"fmt"
)

// 角色記錄 130 bytes/筆。`DEFAULT.DAT` = 780 = 6 × 130，六個預設角色；
// `ROSTER.DAT` 是玩家存檔的名冊。見 docs/formats/02-data-files.md §5。
const RecordSize = 130

// 記錄裡已定位的欄位。
const (
	offName  = 0x00 // 10 bytes，空格填充，第 11 個位元組是 0
	offClass = 0x0F // 職業 0–5
	offStats = 0x10 // 六個屬性，一個 byte 一個
	offAge   = 0x27 // 年齡
	offHP    = 94   // uint16 目前 HP
	offMaxHP = 96   // uint16 HP 上限
	offCur   = 107  // 屬性的第二份：受增減益影響後的當前值
)

// Class 是職業。
type Class byte

const (
	Knight Class = iota
	Paladin
	Archer
	Cleric
	Sorcerer
	Robber
)

var classNames = [...]string{"騎士", "聖騎士", "弓箭手", "牧師", "巫師", "盜賊"}

func (c Class) String() string {
	if int(c) >= len(classNames) {
		return fmt.Sprintf("職業%d", int(c))
	}
	return classNames[c]
}

// Stat 是屬性。順序由六個預設角色反推：每個角色的峰值都落在自己職業
// 該高的那一項（騎士／聖騎士→力量、弓箭手→速度、牧師→性格、
// 巫師→智力、盜賊→準確），六個全中。
type Stat int

const (
	Might Stat = iota
	Intellect
	Personality
	Endurance
	Speed
	Accuracy
	NumStats
)

var statNames = [NumStats]string{"力量", "智力", "性格", "耐力", "速度", "準確"}

func (s Stat) String() string {
	if s < 0 || s >= NumStats {
		return fmt.Sprintf("屬性%d", int(s))
	}
	return statNames[s]
}

// Character 是一個角色。
type Character struct {
	Name  string
	Class Class
	Age   int
	HP    int
	MaxHP int

	// Base 是基礎屬性，Current 是受增減益影響後的值。
	// 兩者在記錄裡各存一份（+0x10 與 +107）。
	Base    [NumStats]int
	Current [NumStats]int

	Raw []byte // 未解的欄位原樣保留，寫回時不能丟
}

// ParseCharacters 解析一份角色檔（DEFAULT.DAT 或 ROSTER.DAT）。
//
// ROSTER.DAT 是 8,293 bytes，不是 130 的整數倍（130×63 + 103），
// 尾端那 103 bytes 不成一筆，略過而不是硬湊。
func ParseCharacters(blob []byte) ([]Character, error) {
	n := len(blob) / RecordSize
	if n == 0 {
		return nil, fmt.Errorf("檔案只有 %d bytes，放不下一筆 %d bytes 的記錄", len(blob), RecordSize)
	}
	out := make([]Character, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, parseCharacter(blob[i*RecordSize:(i+1)*RecordSize]))
	}
	return out, nil
}

func parseCharacter(r []byte) Character {
	name := r[offName : offName+10]
	if k := bytes.IndexByte(name, 0); k >= 0 {
		name = name[:k]
	}
	c := Character{
		Name:  string(bytes.TrimRight(name, " ")),
		Class: Class(r[offClass]),
		Age:   int(r[offAge]),
		HP:    int(r[offHP]) | int(r[offHP+1])<<8,
		MaxHP: int(r[offMaxHP]) | int(r[offMaxHP+1])<<8,
		Raw:   append([]byte(nil), r...),
	}
	for i := Stat(0); i < NumStats; i++ {
		c.Base[i] = int(r[offStats+int(i)])
		c.Current[i] = int(r[offCur+int(i)])
	}
	return c
}

// Empty 回報這一格名冊是不是空的。
func (c Character) Empty() bool { return c.Name == "" }

// Party 是目前的隊伍。原版最多六人。
type Party struct {
	Members []Character
}

// MaxParty 是隊伍人數上限。
const MaxParty = 6
