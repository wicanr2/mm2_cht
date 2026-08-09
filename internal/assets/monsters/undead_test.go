package monsters_test

import (
	"strings"
	"testing"
)

// 不死旗標（`+18` bit 7）：名字明擺著是不死的那幾隻必須全中。
func TestUndeadFlag(t *testing.T) {
	ms := parse(t)
	names := []string{"Zombie", "Skeleton", "Ghost", "Mummy", "Vampire", "Lich", "Wraith", "Spectre", "Wight"}
	checked, flagged := 0, 0
	for _, m := range ms {
		if m.Name == "" {
			continue
		}
		if m.Undead {
			flagged++
		}
		for _, k := range names {
			if strings.Contains(m.Name, k) {
				checked++
				if !m.Undead {
					t.Errorf("%s 沒有不死旗標", m.Name)
				}
				break
			}
		}
	}
	if checked == 0 {
		t.Fatal("名冊裡找不到名字明擺著是不死的怪物")
	}
	// 不死不該是全部，也不該只有那幾隻。
	if flagged < checked || flagged > 100 {
		t.Errorf("有旗標的共 %d 隻，名字對得上的有 %d 隻，量級不對", flagged, checked)
	}
	t.Logf("不死 %d 隻，其中名字對得上的 %d 隻", flagged, checked)
}
