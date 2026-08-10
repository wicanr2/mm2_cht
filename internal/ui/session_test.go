package ui_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

func load(t *testing.T) *ui.Session {
	t.Helper()
	dir := filepath.Join("..", "..", "workplace", "orig", "MM2")
	if _, err := os.Stat(filepath.Join(dir, "MAP.DAT")); err != nil {
		t.Skip("沒有原版資料，跳過")
	}
	// 素材路徑（中文 atlas）是相對於 repo 根目錄找的。
	wd, _ := os.Getwd()
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	s, err := ui.Load(filepath.Join("workplace", "orig", "MM2"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// 開場就要能畫出東西，而且畫的是第一人稱視角不是空畫布。
func TestSessionDrawsSomething(t *testing.T) {
	s := load(t)
	scr := s.Draw()
	scr.Flush()
	nonZero := 0
	for _, v := range scr.Orig.Pix {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero < 1000 {
		t.Errorf("畫面只有 %d 個非背景像素，看起來是空的", nonZero)
	}
}

// 轉向要真的轉，前進要真的走 —— 而且撞牆時位置不變。
func TestMovementChangesState(t *testing.T) {
	s := load(t)
	w := s.World()
	face := w.Face
	if !s.Key(ui.KeyRight) {
		t.Error("右轉沒有回報變化")
	}
	if w.Face == face {
		t.Error("右轉之後朝向沒變")
	}

	// 找一個走得動的方向：四個方向各試一次。
	moved := false
	for i := 0; i < 4 && !moved; i++ {
		x, y := w.X, w.Y
		s.Key(ui.KeyForward)
		if w.X != x || w.Y != y {
			moved = true
			break
		}
		s.Key(ui.KeyConfirm) // 撞牆會留一條訊息，推掉
		s.Key(ui.KeyRight)
	}
	if !moved {
		t.Error("四個方向都走不動")
	}
}

// 訊息模式會攔住方向鍵 —— 原版也是這樣，訊息掛著時走不動。
func TestMessageModeBlocksMovement(t *testing.T) {
	s := load(t)
	// 撞牆一定會產生訊息：先轉到面壁的方向。
	w := s.World()
	for i := 0; i < 4; i++ {
		x, y := w.X, w.Y
		s.Key(ui.KeyForward)
		if w.X == x && w.Y == y && s.Mode == ui.ModeMessage {
			break
		}
		s.Key(ui.KeyConfirm)
		s.Key(ui.KeyRight)
	}
	if s.Mode != ui.ModeMessage {
		t.Skip("這個起點四面都走得動，換一個測法")
	}
	x, y := w.X, w.Y
	if s.Key(ui.KeyForward) {
		t.Error("訊息掛著時方向鍵不該被接受")
	}
	if w.X != x || w.Y != y {
		t.Error("訊息掛著時隊伍卻移動了")
	}
	if !s.Key(ui.KeyConfirm) {
		t.Error("確認鍵沒有推進訊息")
	}
	if s.Mode != ui.ModeExplore {
		t.Errorf("推完訊息之後是 %v，預期回到探索", s.Mode)
	}
}

// 走一段路並把每一格畫出來存成 PNG。
//
// **編譯成功不是視覺測試**（CLAUDE.md §8）——這一支產生的檔案是拿來
// 肉眼比對的，測試本身只保證「每一格都畫得出來、而且畫面確實會變」。
func TestWalkFramesRender(t *testing.T) {
	s := load(t)
	out := filepath.Join("workplace", "gfx", "ui")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	keys := []ui.Key{
		ui.KeyRight, ui.KeyForward, ui.KeyConfirm,
		ui.KeyForward, ui.KeyConfirm,
		ui.KeyLeft, ui.KeyForward, ui.KeyConfirm,
		ui.KeyForward, ui.KeyConfirm,
	}
	var prev []byte
	changed := 0
	for i, k := range keys {
		s.Key(k)
		scr := s.Draw()
		scr.Flush()
		cur := make([]byte, len(scr.Orig.Pix))
		copy(cur, scr.Orig.Pix)
		if prev != nil && !equalBytes(prev, cur) {
			changed++
		}
		prev = cur

		f, err := os.Create(filepath.Join(out, framePath(i, k)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, scr.Hi); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
	if changed == 0 {
		t.Error("走了一整段畫面都沒變過")
	}
	t.Logf("輸出 %d 張到 %s，其中 %d 次畫面有變", len(keys), out, changed)
}

func framePath(i int, k ui.Key) string {
	names := map[ui.Key]string{
		ui.KeyForward: "forward", ui.KeyBack: "back",
		ui.KeyLeft: "left", ui.KeyRight: "right",
		ui.KeyConfirm: "confirm", ui.KeyYes: "yes", ui.KeyNo: "no",
	}
	n := names[k]
	if n == "" {
		n = "key"
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10)) + "-" + n + ".png"
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 施法選單走得完整條路：開選單 → 挑一個真的會法術的人 → 挑法術 →
// 法力真的扣掉。
//
// **不要挑清單的第一個就算數** —— 預設隊伍的第一個施法職業是遊俠，
// 一級時一條法術都不會，挑到他等於什麼都沒驗到。
func TestCastMenuSpendsSP(t *testing.T) {
	s := load(t)
	if !s.Key(ui.KeyCast) {
		t.Fatal("按 C 沒有開選單")
	}
	if s.Mode != ui.ModeMenu || s.Menu == nil || len(s.Menu.Items) == 0 {
		t.Fatalf("按 C 之後是 %v，施法者清單也是空的", s.Mode)
	}

	// 在隊伍裡找一個有法力、而且真的會法術的人。
	want := -1
	for i := range s.Game.Party {
		c := &s.Game.Party[i]
		if c.MaxSP == 0 {
			continue
		}
		for n := 1; n <= 48; n++ {
			if c.Knows(n) {
				want = i
				break
			}
		}
		if want >= 0 {
			break
		}
	}
	if want < 0 {
		t.Skip("隊伍裡沒有會法術的人")
	}
	name := s.Game.Party[want].Name

	// 把游標移到他身上：清單只列得出施法職業，所以要照名字找。
	moved := false
	for i := 0; i < len(s.Menu.Items); i++ {
		if strings.HasPrefix(s.Menu.Items[s.Menu.Cur], name) {
			moved = true
			break
		}
		s.Key(ui.KeyDown)
	}
	if !moved {
		t.Fatalf("施法者清單裡找不到 %s：%v", name, s.Menu.Items)
	}

	s.Key(ui.KeyConfirm) // 選定施法者
	if s.Mode != ui.ModeMenu {
		t.Fatalf("挑完施法者之後是 %v，預期進法術清單", s.Mode)
	}
	if len(s.Menu.Items) == 0 {
		t.Fatal("法術清單是空的")
	}
	before := s.Game.Party[want].SP

	s.Key(ui.KeyConfirm) // 選定法術
	if s.Mode == ui.ModeMenu {
		t.Error("施法之後選單沒有關掉")
	}
	if len(s.Lines) == 0 {
		t.Fatal("施法沒有任何播報")
	}
	after := s.Game.Party[want].SP
	t.Logf("%s 施法：%s（SP %d → %d）", name, s.Lines[0], before, after)
	if after >= before {
		t.Errorf("%s 施完法力沒少（%d → %d）—— 施法沒有真的執行", name, before, after)
	}
}

// 選單的游標會動，而且夾在範圍內不繞回。
func TestMenuCursorClamps(t *testing.T) {
	s := load(t)
	s.Key(ui.KeyCast)
	if s.Menu == nil {
		t.Fatal("選單沒開")
	}
	n := len(s.Menu.Items)
	if n < 2 {
		t.Skip("選項太少，測不出夾住")
	}
	if s.Key(ui.KeyUp) {
		t.Error("已經在第一項還能往上")
	}
	if !s.Key(ui.KeyDown) || s.Menu.Cur != 1 {
		t.Errorf("往下之後游標在 %d，預期 1", s.Menu.Cur)
	}
	for i := 0; i < n+5; i++ {
		s.Key(ui.KeyDown)
	}
	if s.Menu.Cur != n-1 {
		t.Errorf("一直往下之後游標在 %d，預期夾在 %d", s.Menu.Cur, n-1)
	}
	// 取消要關掉選單
	if !s.Key(ui.KeyCancel) || s.Mode == ui.ModeMenu {
		t.Error("取消沒有關掉選單")
	}
}

// 物品欄列得出十二格（裝備六 + 背包六）。
func TestItemMenuListsAllSlots(t *testing.T) {
	s := load(t)
	s.Key(ui.KeyItems)
	if s.Menu == nil {
		t.Fatal("物品選單沒開")
	}
	if len(s.Menu.Items) != 12 {
		t.Errorf("列出 %d 格，預期 12（裝備 6 + 背包 6）", len(s.Menu.Items))
	}
}

// 商店：買得起就扣錢、東西進背包；買不起就不動。
func TestShopBuy(t *testing.T) {
	s := load(t)
	s.Key(ui.KeyShop)
	if s.Mode != ui.ModeMenu || s.Menu == nil {
		t.Fatalf("在地圖 %d 開商店失敗（模式 %v）", s.World().MapIndex, s.Mode)
	}
	if len(s.Menu.Items) == 0 {
		t.Fatal("貨架是空的")
	}
	c := &s.Game.Party[0]
	c.Gold = 100000 // 先給夠錢，測「買得成」那條
	// 清出一個空背包格
	for i := range c.Backpack() {
		c.Items[game.EquippedSlots+i] = game.ItemSlot{}
	}
	gold := c.Gold
	s.Key(ui.KeyConfirm)
	if len(s.Lines) == 0 {
		t.Fatal("買東西沒有播報")
	}
	t.Logf("播報：%s", s.Lines[0])
	if c.Gold >= gold {
		t.Errorf("買完金幣沒少（%d → %d）", gold, c.Gold)
	}
	if c.Backpack()[0].Empty() {
		t.Error("買完背包第一格還是空的")
	}

	// 沒錢就買不成
	s.Key(ui.KeyConfirm) // 推掉訊息
	c.Gold = 0
	s.Key(ui.KeyShop)
	s.Key(ui.KeyConfirm)
	if c.Gold != 0 {
		t.Errorf("沒錢卻扣了錢，剩 %d", c.Gold)
	}
}

// 事件的傳送（opcode 0x0c）經過前端要真的換圖。
//
// 走完整條路徑：ui.Key → Session.Step → World.Trigger → 腳本 → 換圖，
// 而且換完之後前端要重建譯文對照表（對照表是逐段的）。
//
// **要逐一試每個含傳送的事件格**，不能只挑第一個 —— 有些格四面都是牆，
// 從鄰格走不上去，那不代表傳送壞了。
func TestTeleportThroughUI(t *testing.T) {
	s := load(t)
	w := s.World()
	seg := w.EventSegment()
	if seg == nil {
		t.Skip("這張圖沒有事件段")
	}
	s.Answer = true // 有些傳送前面掛著 Y／N

	var cells []int
	for _, ev := range seg.Events {
		i := int(ev.Index) // Scripts[0] 是空的，Index 直接當下標
		if i < 0 || i >= len(seg.Scripts) {
			continue
		}
		sc := seg.Scripts[i]
		for q := 0; q+2 < len(sc); q++ {
			if sc[q] == 0x0c {
				cells = append(cells, int(ev.Cell))
				break
			}
		}
	}
	if len(cells) == 0 {
		t.Skip("段 0 沒有含傳送的事件")
	}

	type dir struct {
		dx, dy int
		face   game.Facing
	}
	dirs := []dir{
		{0, -1, game.North}, {0, 1, game.South},
		{-1, 0, game.East}, {1, 0, game.West},
	}
	start := w.MapIndex
	for _, cell := range cells {
		tx, ty := cell%16, cell/16
		for _, d := range dirs {
			nx, ny := tx+d.dx, ty+d.dy
			if nx < 0 || nx > 15 || ny < 0 || ny > 15 {
				continue
			}
			w.MapIndex, w.X, w.Y, w.Face = start, nx, ny, d.face
			s.Lines, s.Mode = nil, ui.ModeExplore
			s.Key(ui.KeyForward)
			if w.MapIndex == start {
				continue
			}
			t.Logf("從 (%d,%d) 走上 (%d,%d) → 換到地圖 %d", nx, ny, tx, ty, w.MapIndex)
			scr := s.Draw()
			scr.Flush()
			nonZero := 0
			for _, v := range scr.Orig.Pix {
				if v != 0 {
					nonZero++
				}
			}
			if nonZero < 1000 {
				t.Error("換圖之後畫面是空的 —— 譯文表或素材沒跟著換")
			}
			return
		}
	}
	t.Errorf("%d 個含傳送的事件格，沒有一個走得上去", len(cells))
}

// 把三個選單各畫一張出來。
//
// **測試綠燈不等於畫面對** —— 這一支產生的 PNG 是拿來肉眼比對的，
// 測試本身只保證「選單真的蓋上去了」（畫面與沒開選單時不同）。
func TestMenuFramesRender(t *testing.T) {
	s := load(t)
	out := filepath.Join("workplace", "gfx", "ui")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	base := s.Draw()
	plain := make([]byte, len(base.Orig.Pix))
	copy(plain, base.Orig.Pix)

	for _, tc := range []struct {
		key  ui.Key
		name string
	}{
		{ui.KeyCast, "menu-cast"},
		{ui.KeyItems, "menu-items"},
		{ui.KeyShop, "menu-shop"},
	} {
		s.Key(tc.key)
		if s.Mode != ui.ModeMenu {
			t.Errorf("%s 沒有進選單模式", tc.name)
			continue
		}
		scr := s.Draw()
		if equalBytes(plain, scr.Orig.Pix) {
			t.Errorf("%s 開了選單畫面卻沒變", tc.name)
		}
		f, err := os.Create(filepath.Join(out, tc.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, scr.Hi); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		s.Key(ui.KeyCancel)
	}
	t.Logf("三張選單畫面輸出到 %s", out)
}

// 法術清單要顯示說明書上的說明 —— 這個 remake 的目標之一是不必再翻紙本。
func TestSpellMenuShowsManualText(t *testing.T) {
	s := load(t)
	s.Key(ui.KeyCast)
	// 移到會法術的那個人
	for i := 0; i < len(s.Menu.Items); i++ {
		if strings.HasPrefix(s.Menu.Items[s.Menu.Cur], "Gene Eric") {
			break
		}
		s.Key(ui.KeyDown)
	}
	s.Key(ui.KeyConfirm)
	msg := s.Message()
	if msg == "" {
		t.Fatal("法術清單沒有顯示任何說明")
	}
	t.Logf("說明：%s", strings.ReplaceAll(msg, "@", " / "))
	for _, want := range []string{"SP", "／"} {
		if !strings.Contains(msg, want) {
			t.Errorf("說明裡沒有 %q：%q", want, msg)
		}
	}
	// 換一條法術，說明要跟著換。
	first := s.Message()
	if s.Key(ui.KeyDown) && s.Message() == first {
		t.Error("換了法術說明卻沒變")
	}
}

// MM2 沒有「請翻到說明書第 N 段」那種抄寫式防拷。
//
// 這一條是**負面斷言**，所以要守著：哪天發現有，這個測試會先壞掉，
// 而不是讓「遊戲裡查不到的內容」默默漏掉。判準是 EXE 字串裡有沒有
// word／page／paragraph 形式的提問。
func TestNoManualLookupPrompts(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "translations", "strings.json"))
	if err != nil {
		t.Skip("讀不到翻譯檔")
	}
	var doc []struct {
		Key    string `json:"key"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc) < 100 {
		t.Fatalf("只讀到 %d 條字串，正對照失敗", len(doc))
	}
	pat := regexp.MustCompile(`(?i)\bword\b.*\bpage\b|\bparagraph\b|\bpassage number\b|manual page`)
	var hits []string
	for _, s := range doc {
		if pat.MatchString(s.Source) {
			hits = append(hits, s.Key+": "+s.Source)
		}
	}
	if len(hits) > 0 {
		t.Errorf("找到疑似「翻說明書」的提問，要補進遊戲內：\n  %s",
			strings.Join(hits, "\n  "))
	}
}
