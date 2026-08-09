package game

import (
	"encoding/binary"
	"fmt"
)

// Encode 把角色寫回 130 bytes 的記錄格式。
//
// 關鍵是 Raw：解析時整筆原樣留著，寫回時只覆蓋**已經定位的欄位**，
// 其餘二十幾個還沒解的位元組原封不動帶回去。這樣存檔不會因為
// 「remake 還沒解完」就把原版的資料洗掉 —— 也讓存檔能拿回原版讀。
func (c *Character) Encode() []byte {
	out := make([]byte, RecordSize)
	if len(c.Raw) == RecordSize {
		copy(out, c.Raw)
	}
	// 空槽原樣送回去。名冊裡刪掉的角色留著半截舊資料，
	// 名稱欄是全 0 而不是空格填充 —— 照有效角色的規則寫回會改動它們。
	if c.Empty() {
		return out
	}
	// 名稱：10 bytes，空格填充，第 11 個位元組歸零
	for i := 0; i < 10; i++ {
		if i < len(c.Name) {
			out[offName+i] = c.Name[i]
		} else {
			out[offName+i] = ' '
		}
	}
	out[offName+10] = 0

	out[offSex] = byte(c.Sex)
	out[offAlign] = byte(c.Align)
	out[offRace] = byte(c.Race)
	out[offClass] = byte(c.Class)
	out[offLevel] = byte(c.Level)
	out[offAge] = byte(c.Age)
	out[offFood] = byte(c.Food)
	out[offAC] = byte(c.AC)
	out[offGems] = byte(c.Gems)
	out[offGems+1] = byte(c.Gems >> 8)
	writeU32(out, offExp, c.Exp)
	writeU32(out, offGold, c.Gold)
	for i, it := range c.Items {
		out[offItemID+i] = byte(it.ID)
		out[offItemAttr+i] = it.Attr
	}
	for i := 0; i < NumResists; i++ {
		out[offResist+i] = byte(c.Resist[i])
	}
	out[offSL] = byte(c.SL)
	out[offLuck] = byte(c.Luck)
	out[offThief] = byte(c.Thievery)
	out[offLuckB] = byte(c.Luck)
	out[offCond] = c.CondBits
	binary.LittleEndian.PutUint16(out[offHP:], uint16(c.HP))
	binary.LittleEndian.PutUint16(out[offMaxHP:], uint16(c.MaxHP))
	binary.LittleEndian.PutUint16(out[offSP:], uint16(c.SP))
	binary.LittleEndian.PutUint16(out[offMaxSP:], uint16(c.MaxSP))
	for i := Stat(0); i < NumStats; i++ {
		out[offStats+int(i)] = byte(c.Base[i])
		out[offCur+int(i)] = byte(c.Current[i])
	}
	return out
}

// EncodeRoster 把一份名冊寫回去。
//
// 要帶原檔進來，因為尾端有不成一筆的殘餘要原樣保留：
// ROSTER.DAT 是 8,293 bytes = 130 × 63 + **103**，那 103 bytes 不是零，
// 用零填會改動檔案。
func EncodeRoster(cs []Character, orig []byte) ([]byte, error) {
	need := len(cs) * RecordSize
	if len(orig) < need {
		return nil, fmt.Errorf("原檔 %d bytes 放不下 %d 筆記錄", len(orig), len(cs))
	}
	out := make([]byte, len(orig))
	copy(out, orig)
	for i := range cs {
		copy(out[i*RecordSize:], cs[i].Encode())
	}
	return out, nil
}

func writeU32(b []byte, off, v int) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v >> 16)
	b[off+3] = byte(v >> 24)
}
