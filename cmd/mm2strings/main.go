// mm2strings 匯出原版的可翻譯文字，並檢查翻譯檔的完整性。
//
//	go run ./cmd/mm2strings -data workplace/orig/MM2 export translations/strings.json
//	go run ./cmd/mm2strings -data workplace/orig/MM2 check translations/strings.json
//
// 匯出的是 key → {原文, 譯文} 的對照表。key 帶來源與序號，
// 原版檔案更新時才能對得回去。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
	"github.com/wicanr2/mm2_cht/internal/assets/exetext"
	"github.com/wicanr2/mm2_cht/internal/assets/items"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/assets/text"
)

// Entry 是工作檔的一條。工作檔含原版英文全文，不入版控。
type Entry struct {
	Key    string `json:"key"`
	Source string `json:"source"`         // 原文
	Target string `json:"target"`         // 譯文，空字串表示未翻
	Note   string `json:"note,omitempty"` // 給譯者的說明
}

// Translation 是入版控的一條：只有譯文與原文的雜湊。
//
// 原版文字是版權材料，不散布；但譯文必須能驗證「當初翻的是哪一句」，
// 所以留原文的 SHA-256 前 8 碼。原版檔案換版時雜湊對不上就會被 check 抓到。
type Translation struct {
	Key    string `json:"key"`
	Hash   string `json:"src_sha8"`
	Target string `json:"target"`
}

// strHardSplit 是 `STR.DAT` 裡沒有空行、但原版程式碼分開讀的行號。
//
// 全部來自反組譯，不是看內容切的：`sub_17732(N)` 依 `ds:52F4[N]` 決定
// 第 N 群的起點（群 3 ＝ 第 280 行、群 4 ＝ 第 327 行），而 `2SMITH`
// 的 `sub_1D2A4` 又把群 3 的四十七條分成六張指標表，各自在不同時機印。
// 邊界與證據見 `docs/re/05-2smith-control-room.md` §3.1。
var strHardSplit = map[int]bool{
	280: true, // ds:58B8 守門旁白
	284: true, // ds:58C0 中止碼提問
	288: true, // ds:5892 Sheltem 的預錄訊息
	302: true, // ds:5846 要被加密的四行
	306: true, // ds:5868 通關賀詞
	317: true, // ds:587E 戰績、分數與通訊地址
	327: true, // 群 4 的起點：法師公會問候語
}

func srcHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:4])
}

func main() {
	dataDir := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatal("用法: mm2strings [-data dir] <export|check> <strings.json>")
	}
	path := flag.Arg(1)

	entries, err := collect(*dataDir)
	if err != nil {
		log.Fatal(err)
	}

	switch flag.Arg(0) {
	case "export":
		if err := export(path, entries); err != nil {
			log.Fatal(err)
		}
	case "check":
		if err := check(path, entries); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("未知的子命令 %q", flag.Arg(0))
	}
}

func collect(dataDir string) ([]Entry, error) {
	var out []Entry
	for _, f := range []struct{ file, prefix string }{
		{"EVENTSI.DAT", "indoor"},
		{"EVENTSO.DAT", "outdoor"},
	} {
		blob, err := os.ReadFile(findFile(dataDir, f.file))
		if err != nil {
			return nil, err
		}
		segs, err := events.Parse(blob)
		if err != nil {
			return nil, err
		}
		for _, seg := range segs {
			for i, s := range seg.Strings {
				e := Entry{
					Key:    fmt.Sprintf("%s.%02d.%03d", f.prefix, seg.Index, i),
					Source: s,
				}
				if strings.ContainsRune(s, events.LineBreak) {
					e.Note = "'@' 是換行符，譯文要保留斷行位置"
				}
				out = append(out, e)
			}
		}
	}

	// STR.DAT 的長文字（劇情、對話、選單、結局、謎題）。空行是訊息之間的
	// 分隔，跳過但保留行號在 key 裡，翻譯時才對得回原本的段落結構。
	blob, err := os.ReadFile(findFile(dataDir, "STR.DAT"))
	if err != nil {
		return nil, err
	}
	lines, err := text.Parse(blob)
	if err != nil {
		return nil, err
	}
	// 原版每行寬度固定，一句話會被切成好幾行（`B) Soup de Ghoul w/` +
	// `garlic toast`）。逐行翻會翻到殘句，所以用空行分組成訊息再翻，
	// 譯文的換行由 remake 依版面重排。
	//
	// 空行不是唯一的邊界。`strHardSplit` 那幾行原版**沒有**留空行，
	// 但程式碼把它們讀成各自獨立的一張表 —— 不切開的話，
	// 從鐵匠選單到法師公會問候語會黏成同一條，remake 取不到其中一段。
	start, group := 0, []string{}
	flush := func(end int) {
		if len(group) == 0 {
			return
		}
		out = append(out, Entry{
			Key:    fmt.Sprintf("str.%03d", start),
			Source: strings.Join(group, "\n"),
			Note:   fmt.Sprintf("STR.DAT 第 %d–%d 行；換行由 remake 依版面重排", start, end),
		})
		group = nil
	}
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			flush(i - 1)
			continue
		}
		if strHardSplit[i] {
			flush(i - 1)
		}
		if len(group) == 0 {
			start = i
		}
		group = append(group, ln)
	}
	flush(len(lines) - 1)

	// 怪物名（MONSTERS.DAT）與物品名（ITEMS.DAT）。
	mblob, err := os.ReadFile(findFile(dataDir, "MONSTERS.DAT"))
	if err != nil {
		return nil, err
	}
	defs, err := monsters.Parse(mblob)
	if err != nil {
		return nil, err
	}
	for _, m := range defs {
		out = append(out, Entry{Key: fmt.Sprintf("monster.%03d", m.Index), Source: m.Name})
	}
	iblob, err := os.ReadFile(findFile(dataDir, "ITEMS.DAT"))
	if err != nil {
		return nil, err
	}
	list, err := items.Parse(iblob)
	if err != nil {
		return nil, err
	}
	for _, it := range list {
		out = append(out, Entry{Key: fmt.Sprintf("item.%03d", it.Index), Source: it.Name})
	}

	// MM2.EXE 尾部的 DGROUP 初值段：城鎮、職業、種族、陣營、次要技能、
	// 陷阱訊息、戰鬥播報、選單提示都在這裡。key 用 DGROUP 偏移，
	// 抽取規則調整了也不會整批位移。
	exe, err := os.ReadFile(findFile(dataDir, "MM2.EXE"))
	if err != nil {
		return nil, err
	}
	exeStrings, err := exetext.Parse(exe)
	if err != nil {
		return nil, err
	}
	for _, s := range exeStrings {
		out = append(out, Entry{Key: s.Key(), Source: s.Text})
	}
	return out, nil
}

// transPath 是入版控的譯文檔，與工作檔放在同一個目錄。
func transPath(work string) string {
	return filepath.Join(filepath.Dir(work), "zh-Hant.json")
}

func export(path string, entries []Entry) error {
	// 既有譯文從入版控的譯文檔讀回，工作檔可以隨時砍掉重建
	old := map[string]Translation{}
	if b, err := os.ReadFile(transPath(path)); err == nil {
		var prev []Translation
		if err := json.Unmarshal(b, &prev); err == nil {
			for _, e := range prev {
				old[e.Key] = e
			}
		}
	}
	kept, stale := 0, 0
	for i := range entries {
		t, ok := old[entries[i].Key]
		if !ok || t.Target == "" {
			continue
		}
		if t.Hash != srcHash(entries[i].Source) {
			// 原文變了，舊譯文不能直接沿用
			entries[i].Note = strings.TrimSpace(entries[i].Note + " 【原文已變動，需重譯】")
			stale++
			continue
		}
		entries[i].Target = t.Target
		kept++
	}
	if stale > 0 {
		fmt.Printf("警告：%d 條的原文與當初翻譯時不同，已標記需重譯\n", stale)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("匯出 %d 條字串到 %s（沿用既有譯文 %d 條）\n", len(entries), path, kept)
	return writeTranslations(path, entries)
}

// writeTranslations 把工作檔裡的譯文抽成入版控的譯文檔。
// 未翻譯的條目也留著當骨架，但不帶原文。
func writeTranslations(work string, entries []Entry) error {
	out := make([]Translation, 0, len(entries))
	done := 0
	for _, e := range entries {
		out = append(out, Translation{Key: e.Key, Hash: srcHash(e.Source), Target: e.Target})
		if e.Target != "" {
			done++
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	p := transPath(work)
	if err := os.WriteFile(p, append(b, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("譯文檔 %s：%d 條，已翻 %d 條（%.1f%%）\n",
		p, len(out), done, 100*float64(done)/float64(len(out)))
	return nil
}

// check 對得起兩種檔：工作檔（帶 Source）與發布的譯文檔（帶 src_sha8）。
//
// 兩種都要收，因為兩個都會被拿來 check —— 只認一種的話，
// 餵錯檔會得到「2,677 條原文全部對不上」這種看起來像資料壞掉、
// 其實只是欄位不存在的結果。
func check(path string, entries []Entry) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var have []struct {
		Key    string `json:"key"`
		Source string `json:"source"`
		Hash   string `json:"src_sha8"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(b, &have); err != nil {
		return err
	}
	if len(have) == 0 {
		return fmt.Errorf("%s 裡一條都沒有", path)
	}

	type entry struct{ source, hash, target string }
	byKey := map[string]entry{}
	withSource, withHash := 0, 0
	for _, e := range have {
		byKey[e.Key] = entry{e.Source, e.Hash, e.Target}
		if e.Source != "" {
			withSource++
		}
		if e.Hash != "" {
			withHash++
		}
	}
	if withSource == 0 && withHash == 0 {
		return fmt.Errorf("%s 既沒有 source 也沒有 src_sha8，比不了原文", path)
	}

	var missing, drifted, untranslated []string
	for _, e := range entries {
		got, ok := byKey[e.Key]
		if !ok {
			missing = append(missing, e.Key)
			continue
		}
		switch {
		case got.source != "":
			if got.source != e.Source {
				drifted = append(drifted, e.Key)
			}
		case got.hash != "":
			if got.hash != srcHash(e.Source) {
				drifted = append(drifted, e.Key)
			}
		}
		if got.target == "" {
			untranslated = append(untranslated, e.Key)
		}
	}
	sort.Strings(missing)
	sort.Strings(drifted)

	fmt.Printf("原版 %d 條，翻譯檔 %d 條\n", len(entries), len(have))
	fmt.Printf("  未收錄 %d、原文對不上 %d、未翻譯 %d\n",
		len(missing), len(drifted), len(untranslated))
	if len(missing) > 0 {
		fmt.Println("  未收錄範例:", strings.Join(head(missing, 5), ", "))
	}
	if len(drifted) > 0 {
		fmt.Println("  原文對不上:", strings.Join(head(drifted, 5), ", "))
		return fmt.Errorf("翻譯檔與原版不同步")
	}
	return nil
}

func head(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// 原版 EXE 內的檔名是小寫、實體檔案是大寫；Linux/macOS 會分大小寫。
func findFile(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return filepath.Join(dir, name)
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name())
		}
	}
	return filepath.Join(dir, name)
}

