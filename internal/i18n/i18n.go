// Package i18n 載入譯文。
//
// 讀的是入版控的 `translations/zh-Hant.json`（key → 譯文），不是工作檔
// `strings.json`——後者含原版英文全文，不入版控，玩家手上不一定有。
//
// key 的來源與格式見 `cmd/mm2strings`：
//
//	indoor.NN.NNN / outdoor.NN.NNN  事件腳本的字串
//	str.NNN                          STR.DAT 的長文字
//	monster.NNN / item.NNN           怪物名、物品名
//	exe.XXXX                         MM2.EXE 尾部的 UI 與播報文字（XXXX 是 DGROUP 偏移）
package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// DefaultPath 是譯文檔的預設位置。
const DefaultPath = "translations/zh-Hant.json"

// Catalog 是一份譯文。
type Catalog struct {
	byKey map[string]string
}

type entry struct {
	Key    string `json:"key"`
	Target string `json:"target"`
}

// Load 讀入譯文檔。檔案不存在時回一份空的 Catalog 而不是錯誤 ——
// 沒有譯文就顯示原文，遊戲還是能跑。
func Load(path string) (*Catalog, error) {
	c := &Catalog{byKey: map[string]string{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	var rows []entry
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("解析 %s: %w", path, err)
	}
	for _, r := range rows {
		if r.Target != "" {
			c.byKey[r.Key] = r.Target
		}
	}
	return c, nil
}

// Len 回傳有譯文的條目數。
func (c *Catalog) Len() int { return len(c.byKey) }

// T 回傳指定 key 的譯文，沒有就回空字串。
func (c *Catalog) T(key string) string { return c.byKey[key] }

// Or 回傳譯文，沒有就回 fallback。顯示用的入口大多走這個。
func (c *Catalog) Or(key, fallback string) string {
	if s := c.byKey[key]; s != "" {
		return s
	}
	return fallback
}

// Exe 回傳 MM2.EXE 尾部某個 DGROUP 偏移的譯文。
func (c *Catalog) Exe(offset int) string { return c.byKey[fmt.Sprintf("exe.%04X", offset)] }

// SourceMap 把「原文 → 譯文」的對照表組出來，給只有原文在手的呼叫端用
// （怪物名就是這種：戰鬥流程拿到的是 `MONSTERS.DAT` 的英文名）。
//
// sources 是 key → 原文。同一段原文在不同 key 下有不同譯文時，先出現的贏 ——
// 這種情況本身要視為譯名不一致，該回頭統一，不是靠這裡的順序解決。
func (c *Catalog) SourceMap(sources map[string]string, prefix string) map[string]string {
	out := map[string]string{}
	for key, src := range sources {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		t := c.byKey[key]
		if t == "" || src == "" {
			continue
		}
		if _, dup := out[src]; !dup {
			out[src] = t
		}
	}
	return out
}
