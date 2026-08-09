package game_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/gamedata"
)

// 測試跑在套件目錄下，資料在 repo 根目錄的 data/。
func TestMain(m *testing.M) {
	dir := gamedata.Dir()
	if _, err := os.Stat(dir); err != nil {
		dir = filepath.Join("..", "..", "data")
	}
	d, err := gamedata.Load(dir)
	if err != nil {
		panic(err)
	}
	game.UseData(d)
	os.Exit(m.Run())
}
