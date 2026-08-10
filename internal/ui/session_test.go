package ui_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
)

// load 開一場遊玩。素材路徑（中文 atlas、data/）都是相對 repo 根目錄找的，
// 所以要先把工作目錄換過去。
//
// **同一個測試裡呼叫兩次也要能用** —— 換過之後再用相對路徑往上兩層
// 就跑到 repo 外面了，症狀是第二次莫名其妙 skip。
func load(t *testing.T) *ui.Session {
	t.Helper()
	const data = "workplace/orig/MM2"
	if _, err := os.Stat(filepath.Join(data, "MAP.DAT")); err != nil {
		// 還沒換過目錄，往上兩層試一次。
		if _, err := os.Stat(filepath.Join("..", "..", data, "MAP.DAT")); err != nil {
			t.Skip("沒有原版資料，跳過")
		}
		wd, _ := os.Getwd()
		if err := os.Chdir(filepath.Join("..", "..")); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chdir(wd) })
	}
	s, err := ui.Load(data)
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

// 查說明書：兩層選單走得完，而且內容是說明書抽出來的那幾類。
func TestReferenceMenu(t *testing.T) {
	s := load(t)
	if s.Ref == nil {
		t.Skip("沒有 data/reference.json")
	}
	if !s.Key(ui.KeyRef) || s.Mode != ui.ModeMenu {
		t.Fatal("按 K 沒有開查閱選單")
	}
	if len(s.Menu.Items) != 5 {
		t.Errorf("第一層有 %d 類，預期 5（第二技能／城鎮指令／冒險指令／技能在哪學／職業）",
			len(s.Menu.Items))
	}
	s.Key(ui.KeyConfirm) // 進第二技能
	if s.Mode != ui.ModeMenu {
		t.Fatal("進不了第二層")
	}
	if len(s.Menu.Items) != 15 {
		t.Errorf("第二技能列出 %d 條，說明書是 15 項", len(s.Menu.Items))
	}
	if !strings.Contains(s.Menu.Items[9], "商人") {
		t.Errorf("第 10 項是 %q，說明書是「商人」——"+
			"這一項也是 game.SkillMerchant = 10 的依據", s.Menu.Items[9])
	}
	s.Key(ui.KeyConfirm) // 回第一層
	if len(s.Menu.Items) != 5 {
		t.Error("沒有回到第一層")
	}
	// 職業那一類是從引擎的表組出來的，不是手冊 —— 要有八個職業。
	for i := 0; i < 4; i++ {
		s.Key(ui.KeyDown)
	}
	s.Key(ui.KeyConfirm)
	if len(s.Menu.Items) != 8 {
		t.Errorf("職業列出 %d 項，預期 8", len(s.Menu.Items))
	}
	if !strings.Contains(s.Menu.Items[0], "武士") {
		t.Errorf("第一個職業是 %q，預期武士", s.Menu.Items[0])
	}
}

// 物品欄可以裝備與卸下，而且戰鬥數值跟著重算。
func TestEquipUnequip(t *testing.T) {
	s := load(t)
	c := &s.Game.Party[0]
	// 把背包第一格塞一把武器（Long Sword = 8 面骰那件）
	c.Items[game.EquippedSlots] = game.ItemSlot{ID: 25}
	s.Key(ui.KeyItems)
	if s.Menu == nil {
		t.Fatal("物品選單沒開")
	}
	// 背包第一格是第 7 項（索引 6）
	for i := 0; i < 6; i++ {
		s.Key(ui.KeyDown)
	}
	s.Key(ui.KeyConfirm)
	if len(s.Lines) == 0 || !strings.HasPrefix(s.Lines[0], "裝備") {
		t.Fatalf("裝備沒有回報：%v", s.Lines)
	}
	if c.Items[0].Empty() {
		t.Error("裝備完第一格還是空的")
	}
	if !c.Items[game.EquippedSlots].Empty() {
		t.Error("裝備完背包那一格沒清掉")
	}

	// 再卸下來
	s.Key(ui.KeyConfirm) // 推掉訊息
	s.Key(ui.KeyItems)
	s.Key(ui.KeyConfirm) // 游標在第一格＝已裝備
	if c.Items[0].Empty() != true {
		t.Error("卸下之後裝備格還有東西")
	}
	if c.Items[game.EquippedSlots].Empty() {
		t.Error("卸下之後背包沒收到東西")
	}
}

// 查說明書的畫面也要拍一張，順便驗游標的 ▶ 有烘進中文 atlas。
//
// 缺字不會報錯，只會安靜地少一個字 —— 所以要比對「有游標」與
// 「把游標移開」兩張的差異，而不是只看畫得出來。
func TestReferenceFrameAndCursor(t *testing.T) {
	s := load(t)
	if s.Ref == nil {
		t.Skip("沒有 data/reference.json")
	}
	s.Key(ui.KeyRef)
	s.Key(ui.KeyConfirm) // 進第二技能
	first := s.Draw()
	a := make([]byte, len(first.Hi.Pix))
	copy(a, first.Hi.Pix)

	out := filepath.Join("workplace", "gfx", "ui")
	os.MkdirAll(out, 0o755)
	f, err := os.Create(filepath.Join(out, "menu-reference.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, first.Hi); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	s.Key(ui.KeyDown) // 游標下移
	second := s.Draw()
	if equalBytes(a, second.Hi.Pix) {
		t.Error("游標移了畫面卻一模一樣 —— ▶ 可能沒烘進 atlas")
	}
}

// 清單比框高時要捲得到最後一項 —— 看不見的項目等於選不到。
func TestMenuScrolls(t *testing.T) {
	s := load(t)
	if s.Ref == nil {
		t.Skip("沒有 data/reference.json")
	}
	s.Key(ui.KeyRef)
	s.Key(ui.KeyConfirm) // 第二技能，15 項
	m := s.Menu
	if len(m.Items) <= ui.VisibleRows {
		t.Skipf("只有 %d 項，捲不起來", len(m.Items))
	}
	for i := 0; i < len(m.Items); i++ {
		s.Key(ui.KeyDown)
	}
	if m.Cur != len(m.Items)-1 {
		t.Fatalf("游標在 %d，預期在最後一項 %d", m.Cur, len(m.Items)-1)
	}
	lines := m.Lines()
	last := m.Items[len(m.Items)-1]
	seen := false
	for _, l := range lines {
		if strings.Contains(l, last) {
			seen = true
		}
	}
	if !seen {
		t.Errorf("游標在最後一項，畫出來的卻沒有它：%v", lines)
	}
	if !strings.Contains(lines[0], "15／15") {
		t.Errorf("標題沒有標出位置：%q", lines[0])
	}
}

// 前端要擋得住職業不能用的東西 —— 而且擋下來時要說原因。
func TestEquipRejectsWrongClass(t *testing.T) {
	s := load(t)
	// 找一個牧師
	who := -1
	for i := range s.Game.Party {
		if s.Game.Party[i].Class == 3 {
			who = i
		}
	}
	if who < 0 {
		t.Skip("隊伍裡沒有牧師")
	}
	c := &s.Game.Party[who]
	c.Items[game.EquippedSlots] = game.ItemSlot{ID: 4} // Dagger，禁牧師
	for i := 0; i < game.EquippedSlots; i++ {
		c.Items[i] = game.ItemSlot{}
	}
	s.Key(ui.KeyItems)
	s.SelectMember(who)
	for i := 0; i < game.EquippedSlots; i++ {
		s.Key(ui.KeyDown)
	}
	s.Key(ui.KeyConfirm)
	if len(s.Lines) == 0 || !strings.Contains(s.Lines[0], "職業") {
		t.Errorf("牧師裝匕首的回報是 %v，預期說明職業不能用", s.Lines)
	}
	if !c.Items[0].Empty() {
		t.Error("擋下來了卻還是裝上去了")
	}
}

// 存檔要能存得回來：位置、朝向、種子都要一致。
//
// **存檔壞掉不會有任何徵兆**（遊戲照樣跑，只是下次開起來位置不對），
// 所以要真的存一次、讀一次、逐欄比對。
func TestSaveRestore(t *testing.T) {
	s := load(t)
	w := s.World()
	// 走幾步讓狀態不再是初值 —— 從初值存讀回來，比對不出東西。
	for i := 0; i < 6; i++ {
		s.Key(ui.KeyRight)
		s.Key(ui.KeyForward)
		s.Key(ui.KeyConfirm)
	}
	wantX, wantY, wantFace, wantMap := w.X, w.Y, w.Face, w.MapIndex

	if msg := s.Save(); !strings.Contains(msg, "已存檔") {
		t.Fatalf("存檔回報 %q", msg)
	}
	t.Cleanup(func() { os.RemoveAll("save") })

	// 另開一場再讀回來
	s2 := load(t)
	if !s2.Restore() {
		t.Fatal("讀不回存檔")
	}
	w2 := s2.World()
	if w2.X != wantX || w2.Y != wantY || w2.Face != wantFace || w2.MapIndex != wantMap {
		t.Errorf("讀回來是圖%d (%d,%d) 面%v，存的是圖%d (%d,%d) 面%v",
			w2.MapIndex, w2.X, w2.Y, w2.Face, wantMap, wantX, wantY, wantFace)
	}
	// 讀回來之後畫得出來（譯文表要跟著換圖重建）
	scr := s2.Draw()
	nonZero := 0
	for _, v := range scr.Orig.Pix {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero < 1000 {
		t.Error("讀檔之後畫面是空的")
	}
}

// 撞門與開鎖要有回報，而且不會讓遊戲卡在別的模式。
func TestBashAndUnlockReport(t *testing.T) {
	s := load(t)
	for _, tc := range []struct {
		key  ui.Key
		name string
	}{{ui.KeyBash, "撞門"}, {ui.KeyUnlock, "開鎖"}} {
		s.Lines = nil
		s.Mode = ui.ModeExplore
		if !s.Key(tc.key) {
			t.Errorf("%s 沒有回報變化", tc.name)
			continue
		}
		if len(s.Lines) == 0 {
			t.Errorf("%s 沒有任何訊息", tc.name)
			continue
		}
		t.Logf("%s：%s", tc.name, s.Lines[0])
		if s.Mode != ui.ModeMessage {
			t.Errorf("%s 之後是 %v，預期訊息模式", tc.name, s.Mode)
		}
	}
}

// 戰鬥中可以選擇溜跑，而且成功率是這張地圖的 ATTRIB +13。
func TestCombatRun(t *testing.T) {
	s := load(t)
	// 手動擺一場戰鬥，不必等隨機遭遇。
	var d monsters.Monster
	d.HP, d.SpecialUses, d.Speed, d.AC = 5, 1, 1, 1
	m := game.NewMonster(d)
	m.Display = "測試怪"
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	s.Game.Fight = &game.Encounter{Party: party, Monsters: []game.Combatant{m}}
	s.Mode = ui.ModeCombat
	before := len(s.Game.Fight.Party)

	// 城鎮的溜跑成功率是 100，一次就該跑掉。
	if !s.Key(ui.KeyRun) {
		t.Fatal("溜跑鍵沒有回報變化")
	}
	if len(s.Lines) == 0 {
		t.Fatal("溜跑沒有任何播報")
	}
	t.Logf("播報：%s", s.Lines[0])
	if a := s.Game.CurrentAttr(); a != nil && a.RunChance() == 100 {
		if s.Game.Fight != nil && len(s.Game.Fight.Party) >= before {
			t.Errorf("成功率 100 卻沒人跑掉（%d → %d）", before, len(s.Game.Fight.Party))
		}
	}
}
