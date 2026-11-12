// mm2promo streams a controlled battle demonstration through the real UI.
// Run only inside Docker. stdout is 960x640 RGBA at 30 fps; no save is written.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

func main() {
	data := flag.String("data", "workplace/orig/MM2", "original data")
	root := flag.String("resources", ".", "packaged resources")
	flag.Parse()
	s, err := ui.Load(*data)
	if err != nil {
		log.Fatal(err)
	}
	if err = s.InitPresentation(*root, "", ""); err != nil {
		log.Fatal(err)
	}
	p := s.Presentation()
	p.HD = true
	if err = s.SetPresentation(p); err != nil {
		log.Fatal(err)
	}
	s.Game.Rand = game.NewRand(0x1111)
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	var enemies []game.Combatant
	for _, index := range []int{3, 4, 5} {
		def := s.Game.Bestiary[index]
		m := game.NewMonster(def)
		m.Display = s.Game.Names[def.Name]
		enemies = append(enemies, m)
	}
	s.Game.Fight = &game.Encounter{Party: party, Monsters: enemies, Front: 3}
	s.Mode = ui.ModeCombat
	bar := ui.NewToolbar(s)
	for frame := 0; frame < 360; frame++ {
		if frame%4 == 0 {
			s.Tick()
		}
		if frame == 75 || frame == 180 || frame == 285 {
			s.Key(ui.KeyConfirm)
		}
		if frame == 105 || frame == 210 || frame == 315 {
			s.PressDigit(1)
		}
		im := bar.Frame(s.Draw())
		if _, err = os.Stdout.Write(im.Pix); err != nil {
			log.Fatal(err)
		}
	}
}
