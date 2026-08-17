// mm2cell 印出某一格附近每一面牆「畫出來長什麼樣」。
//
//	go run ./cmd/mm2cell -map 0 -x 9 -y 4
//
// 輸出四個方向各一個記號：`.` 沒有東西、`#` 實牆、`D` 門、`T` 種類 3
// （石牆＋火炬）。y 由上往下遞減印，與遊戲內的方位一致（北是 +y）。
//
// 為什麼需要它：第一人稱畫面對不上原版時，第一個要分開的問題是
// 「牆的資料是什麼」與「畫得對不對」。少了這一步就只能對著畫面猜，
// 而**猜錯的方向與猜對的方向在畫面上長得一樣**。
// 這支只讀 `MAP.DAT`／`ATTRIB.DAT`，不畫圖，所以答案與繪圖層無關。
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

func main() {
	dataDir := flag.String("data", "workplace/orig/MM2", "原版資料目錄")
	mapIdx := flag.Int("map", 0, "地圖編號")
	x := flag.Int("x", 7, "中心格的 X")
	y := flag.Int("y", 3, "中心格的 Y")
	r := flag.Int("r", 2, "印出中心格周圍幾格")
	flag.Parse()

	s, err := ui.Load(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	w := s.Game.World
	w.MapIndex = *mapIdx
	m := w.CurrentMap()
	if m == nil {
		log.Fatalf("載不到地圖 %d", *mapIdx)
	}
	name := [4]string{"N", "E", "S", "W"}
	mark := func(k game.WallKind) string {
		switch k {
		case game.WallNone:
			return "."
		case game.WallSolid:
			return "#"
		case game.WallDoor:
			return "D"
		}
		return "T"
	}
	for cy := *y + *r; cy >= *y-*r; cy-- {
		for cx := *x - *r; cx <= *x+*r; cx++ {
			if game.Cell(cx, cy) < 0 {
				continue
			}
			line := fmt.Sprintf("(%2d,%2d) ", cx, cy)
			for f := 0; f < 4; f++ {
				line += " " + name[f] + mark(m.DrawKind(cx, cy, game.Facing(f)))
			}
			fmt.Println(line)
		}
		fmt.Println()
	}
}
