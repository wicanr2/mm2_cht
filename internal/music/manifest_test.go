package music

import (
	"fmt"
	"strings"
	"testing"
)

func fullTracks() string {
	parts := make([]string, 0, len(allRoles))
	for _, role := range allRoles {
		parts = append(parts, fmt.Sprintf("%q:%q", role, string(role)+".ogg"))
	}
	return strings.Join(parts, ",")
}

func TestParseMegaDriveRequiresCompletePack(t *testing.T) {
	data := []byte(`{"theme":"megadrive","tracks":{"intro":"intro.ogg"}}`)
	if _, err := ParseManifest(data, "/packs/md", ThemeMegaDrive); err == nil {
		t.Fatal("不完整 Mega Drive 包應失敗")
	}
	data = []byte(`{"theme":"megadrive","tracks":{` + fullTracks() + `}}`)
	pack, err := ParseManifest(data, "/packs/md", ThemeMegaDrive)
	if err != nil {
		t.Fatal(err)
	}
	if !pack.Complete || len(pack.Tracks) != 16 {
		t.Fatalf("完整性錯誤：complete=%v tracks=%d", pack.Complete, len(pack.Tracks))
	}
}

func TestOtherThemesExposeIncompleteStatus(t *testing.T) {
	pack, err := ParseManifest([]byte(`{"theme":"msx","tracks":{"town":"town.ogg"}}`), "/packs/msx", ThemeMSX)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Complete {
		t.Fatal("部分 MSX 包不可標示為完整")
	}
}

func TestRejectThemeRoleAndUnsafePath(t *testing.T) {
	tests := []string{
		`{"theme":"wav","tracks":{}}`,
		`{"theme":"off","tracks":{"unknown":"x.ogg"}}`,
		`{"theme":"off","tracks":{"town":"../x.ogg"}}`,
		`{"theme":"off","tracks":{"town":"/tmp/x.ogg"}}`,
		`{"theme":"off","tracks":{"town":"C:\\x.ogg"}}`,
		`{"theme":"off","tracks":{"town":"x\\y.ogg"}}`,
	}
	for _, data := range tests {
		if _, err := ParseManifest([]byte(data), "/packs", ""); err == nil {
			t.Errorf("應拒絕 %s", data)
		}
	}
	if _, err := ParseManifest([]byte(`{"theme":"msx","tracks":{}}`), "/packs", ThemeAmiga); err == nil {
		t.Fatal("指定主題不符應失敗")
	}
}

func TestRejectDuplicateRole(t *testing.T) {
	data := []byte(`{"theme":"off","tracks":{"town":"a.ogg","town":"b.ogg"}}`)
	if _, err := ParseManifest(data, "/packs", ThemeOff); err == nil {
		t.Fatal("重複角色應失敗")
	}
}

func TestResolvedPathsAreRelativeToBase(t *testing.T) {
	pack, err := ParseManifest([]byte(`{"theme":"amiga","tracks":{"town":"audio/town.ogg"}}`), "/packs/amiga", ThemeAmiga)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pack.Tracks[Town], "/packs/amiga/audio/town.ogg"; got != want {
		t.Fatalf("路徑=%q，預期 %q", got, want)
	}
}
