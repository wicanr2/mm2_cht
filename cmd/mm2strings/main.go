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
		if len(group) == 0 {
			start = i
		}
		group = append(group, ln)
	}
	flush(len(lines) - 1)
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

func check(path string, entries []Entry) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var have []Entry
	if err := json.Unmarshal(b, &have); err != nil {
		return err
	}
	byKey := map[string]Entry{}
	for _, e := range have {
		byKey[e.Key] = e
	}

	var missing, drifted, untranslated []string
	for _, e := range entries {
		got, ok := byKey[e.Key]
		if !ok {
			missing = append(missing, e.Key)
			continue
		}
		if got.Source != e.Source {
			drifted = append(drifted, e.Key)
		}
		if got.Target == "" {
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

