// Package music 提供不含原版內容的本機音樂包 manifest 讀取與驗證。
package music

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Theme 是音樂包所標示的音源主題。
type Theme string

const (
	ThemeMegaDrive Theme = "megadrive"
	ThemeMSX       Theme = "msx"
	ThemeAmiga     Theme = "amiga"
	ThemeDOS       Theme = "dos"
	ThemeOff       Theme = "off"
)

// Role 是遊戲中可替換的音樂角色。
type Role string

const (
	Intro        Role = "intro"
	Town         Role = "town"
	Battle       Role = "battle"
	EnemyKilled  Role = "enemy_killed"
	Victory      Role = "victory"
	MemberKilled Role = "member_killed"
	Defeat       Role = "defeat"
	Training     Role = "training"
	Temple       Role = "temple"
	Blacksmith   Role = "blacksmith"
	Inn          Role = "inn"
	Tavern       Role = "tavern"
	Dungeon      Role = "dungeon"
	Outside      Role = "outside"
	Treasure     Role = "treasure"
	Castle       Role = "castle"
)

var allRoles = []Role{Intro, Town, Battle, EnemyKilled, Victory, MemberKilled, Defeat, Training, Temple, Blacksmith, Inn, Tavern, Dungeon, Outside, Treasure, Castle}

// Pack 是已驗證的音樂包。Tracks 的值是相對於 manifest 所在目錄解析後的本機路徑。
// Complete 僅在十六個角色皆有映射時為真；非 Mega Drive 的部分包仍可合法載入。
type Pack struct {
	Theme    Theme
	Tracks   map[Role]string
	Complete bool
}

type rawManifest struct {
	Theme  string            `json:"theme"`
	Tracks map[string]string `json:"tracks"`
}

// ParseManifest 驗證並解析 JSON。baseDir 是音樂包根目錄，不會檢查音檔是否已存在。
// 呼叫端可用 Pack.Tracks 逐一開啟音檔；這使 manifest 驗證與部署檔案檢查保持分離。
func ParseManifest(data []byte, baseDir string, requested Theme) (*Pack, error) {
	if err := rejectDuplicateTrackKeys(data); err != nil {
		return nil, err
	}
	var raw rawManifest
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析音樂 manifest 失敗：%w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("音樂 manifest 含有多個 JSON 值")
	}
	theme := Theme(strings.ToLower(strings.TrimSpace(raw.Theme)))
	if !validTheme(theme) {
		return nil, fmt.Errorf("未知音樂主題 %q", raw.Theme)
	}
	if requested != "" && Theme(strings.ToLower(strings.TrimSpace(string(requested)))) != theme {
		return nil, fmt.Errorf("音樂主題不符：manifest 為 %q，指定為 %q", theme, requested)
	}
	tracks := make(map[Role]string, len(raw.Tracks))
	for name, rel := range raw.Tracks {
		role := Role(name)
		if !isRole(role) {
			return nil, fmt.Errorf("未知音樂角色 %q", name)
		}
		clean, err := safeRelativePath(rel)
		if err != nil {
			return nil, fmt.Errorf("角色 %q 的音檔路徑無效：%w", name, err)
		}
		tracks[role] = filepath.Join(baseDir, filepath.FromSlash(clean))
	}
	complete := len(tracks) == len(allRoles)
	if theme == ThemeMegaDrive && !complete {
		return nil, fmt.Errorf("Mega Drive 音樂包必須完整包含 %d 個角色（目前 %d 個）", len(allRoles), len(tracks))
	}
	return &Pack{Theme: theme, Tracks: tracks, Complete: complete}, nil
}

// LoadManifestFile 讀取 manifest 檔案；音檔路徑以該檔案所在目錄為基準。
func LoadManifestFile(path string, requested Theme) (*Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("讀取音樂 manifest %q 失敗：%w", path, err)
	}
	return ParseManifest(data, filepath.Dir(path), requested)
}

// LoadManifest 是 LoadManifestFile 的簡短別名，供播放器初始化使用。
func LoadManifest(path string, requested Theme) (*Pack, error) {
	return LoadManifestFile(path, requested)
}

func validTheme(t Theme) bool {
	return t == ThemeMegaDrive || t == ThemeMSX || t == ThemeAmiga || t == ThemeDOS || t == ThemeOff
}

func isRole(r Role) bool {
	for _, known := range allRoles {
		if r == known {
			return true
		}
	}
	return false
}

func safeRelativePath(value string) (string, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return "", fmt.Errorf("路徑不可為空或含前後空白")
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) || (len(value) >= 2 && value[1] == ':') {
		return "", fmt.Errorf("不可使用絕對路徑")
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return "", fmt.Errorf("路徑不可為空")
	}
	for _, part := range parts {
		if part == ".." {
			return "", fmt.Errorf("不可含有路徑穿越")
		}
	}
	// 以 slash 統一儲存，並拒絕會在不同平台產生歧義的反斜線。
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("不可使用反斜線路徑")
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("不可含有路徑穿越")
	}
	return clean, nil
}

func rejectDuplicateTrackKeys(data []byte) error {
	var doc struct {
		Tracks json.RawMessage `json:"tracks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	if len(doc.Tracks) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(doc.Tracks))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil
	}
	seen := map[string]bool{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil
		}
		name, ok := key.(string)
		if !ok {
			return nil
		}
		if seen[name] {
			return fmt.Errorf("音樂角色重複定義 %q", name)
		}
		seen[name] = true
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil
		}
	}
	return nil
}
