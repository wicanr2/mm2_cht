package ui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 介面訊息用到的字都要在 atlas 裡。
//
// 缺字**不會報錯**，只會在畫面上安靜地少一個字（「場景素材」變成
// 「場景素　」）。字型是從原始碼的字串常值烘出來的
// （`tools/build_cjk_font.py`），所以這個測試同時守住兩件事：
// 新訊息用了沒烘的字、以及改完訊息忘記重跑烘字。
//
// 用 go/parser 而不是掃文字：註解整份是中文，用正規表示式分不開
// 「訊息裡的字」與「註解裡的字」—— 分不開就只能把註解也烘進去，
// atlas 會多出上千個畫面上永遠不會出現的字。
func TestUIStringsHaveGlyphs(t *testing.T) {
	s := load(t)
	f := s.Assets.CJK
	if f == nil {
		t.Skip("沒有中文 atlas")
	}
	fset := token.NewFileSet()
	seen := map[rune]bool{}
	var miss []rune
	// 往上找到 go.mod 才是專案根；容器裡把 repo 掛在哪不一定，
	// 寫死 `../..` 會在掛載點不同時走到容器的檔案系統根目錄去。
	root := "."
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Join("..", root)
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "workplace", ".git", "docs", "translations":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return nil // 解不開就跳過，語法錯誤自然會在編譯時抓到
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			for _, r := range lit.Value {
				if r > 0x7F && !seen[r] && len(f.Missing(string(r))) > 0 {
					seen[r] = true
					miss = append(miss, r)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("掃原始碼失敗：%v", err)
	}
	if len(miss) > 0 {
		t.Errorf("atlas 缺 %d 個字：%q（重跑 tools/build_cjk_font.py）", len(miss), string(miss))
	}
}
