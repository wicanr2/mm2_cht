package ui_test

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/wicanr2/mm2_cht/internal/assets/events"
	"github.com/wicanr2/mm2_cht/internal/assets/monsters"
	"github.com/wicanr2/mm2_cht/internal/game"
	"github.com/wicanr2/mm2_cht/internal/ui"
	"github.com/wicanr2/mm2_cht/internal/view"
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

// 新局要落在原版從角色選擇畫面離開後的第一人稱起點，不是 World 的 Go 零值。
// DOSBox dump：地圖 0（Middlegate）、(7,3)、面北；見 docs/playtest/01 §7。
func TestFreshLoadUsesOracleMiddlegateStart(t *testing.T) {
	s := load(t)
	w := s.World()
	if w.MapIndex != 0 || w.X != 7 || w.Y != 3 || w.Face != game.North {
		t.Errorf("新局起點是圖%d (%d,%d) 面%v，預期圖0 (7,3) 面北",
			w.MapIndex, w.X, w.Y, w.Face)
	}
	if len(s.Game.Party) != 6 {
		t.Errorf("新局隊伍有 %d 人，預期 DEFAULT.DAT 的 6 人", len(s.Game.Party))
	}
}

// 從原版新局起點一路按一般移動鍵，可以進入已由 DOSBox 驗過的神殿，
// 而且離開後不會在下一格又重開它。原版路徑是：北走三步，第 2 步的
// (7,5) 旅店招牌因為 `Kind = 0x20`（只認面南）**不會出現**，第 3 步的
// (7,6) 才畫出 `Gateway Temple`；第 4 步進神殿、拒絕，下一步正常離開。
// 見 docs/playtest/01 §6 的重測表：那三步 DOSBox 逐格量過。
func TestFreshStartReachesTempleThroughUI(t *testing.T) {
	s := load(t)
	if !s.Key(ui.KeyForward) {
		t.Fatal("第 1 步的前進鍵沒有作用")
	}
	if s.Mode != ui.ModeExplore {
		t.Fatalf("第 1 步後是 %v，原版此時仍在探索", s.Mode)
	}
	for step := 2; step <= 3; step++ {
		if !s.Key(ui.KeyForward) {
			t.Fatalf("第 %d 步的前進鍵沒有作用", step)
		}
		if s.Mode != ui.ModeExplore {
			t.Fatalf("第 %d 步後是 %v，原版招牌不會攔住輸入", step, s.Mode)
		}
	}
	if s.Message() == "" {
		t.Fatal("第 3 步的 Gateway Temple 招牌沒有顯示")
	}
	if !s.Key(ui.KeyForward) {
		t.Fatal("走進神殿的前進鍵沒有作用")
	}
	if s.Game.Facility != game.FacilityTemple || s.Mode != ui.ModeMenu {
		t.Fatalf("第四步後是設施 %v、模式 %v，預期神殿選單", s.Game.Facility, s.Mode)
	}
	// N／拒絕是正常離開神殿的路徑；現行 UI 將設施提示放進訊息佇列，
	// 因而在關選單後再確認一次。
	if !s.Key(ui.KeyNo) {
		t.Fatal("神殿選單無法以拒絕鍵離開")
	}
	for n := 0; s.Mode == ui.ModeMessage && n < 16; n++ {
		if !s.Key(ui.KeyConfirm) {
			t.Fatal("神殿提示無法確認")
		}
	}
	if s.Mode != ui.ModeExplore {
		t.Fatalf("離開神殿後是 %v，預期探索模式", s.Mode)
	}
	beforeX, beforeY := s.World().X, s.World().Y
	if !s.Key(ui.KeyForward) {
		t.Fatal("離開神殿後的前進鍵沒有作用")
	}
	if s.World().X == beforeX && s.World().Y == beforeY {
		t.Fatal("離開神殿後前進卻沒有移動")
	}
	if s.Game.Facility != game.FacilityNone || s.Mode == ui.ModeMenu {
		t.Errorf("離開神殿後下一格仍是設施 %v、模式 %v，設施狀態被錯誤保留",
			s.Game.Facility, s.Mode)
	}
}

// 事件 Y/N 必須在踩到事件後才顯示並收答案。這條直接守住先前的缺口：
// 探索模式按 Y/N 曾經只是替「下一個」提問預先設值，腳本早就跑完了。
func TestEventYesNoWaitsForPlayerInput(t *testing.T) {
	s := load(t)
	if s.Key(ui.KeyYes) {
		t.Error("探索模式的 Y 不該再替下一題預設答案")
	}
	w := s.World()
	w.MapIndex, w.X, w.Y, w.Face = 0, 7, 3, game.North
	w.SetGlobal(0, 0)
	seg := w.EventSegment()
	m := w.CurrentMap()
	if seg == nil || m == nil {
		t.Fatal("找不到 Middlegate 事件資料")
	}
	cell := game.Cell(7, 4)
	if len(seg.Scripts) >= 256 || len(seg.Strings) >= 255 {
		t.Fatal("測試事件索引超出原版一位元組範圍")
	}
	seg.Strings = append(seg.Strings, "Continue the test (y/n)?")
	stringIndex := byte(len(seg.Strings))
	scriptIndex := byte(len(seg.Scripts))
	seg.Scripts = append(seg.Scripts, []byte{
		game.OpShowString, stringIndex,
		game.OpAsk,
		game.OpSkipIfPaid, 1,
		game.OpWriteGlobal, 0, 0x10, // N 分支
		game.OpSkipIfUnpaid, 1,
		game.OpWriteGlobal, 0, 0x20, // Y 分支
		0,
	})
	found := false
	for i := range seg.Events {
		if int(seg.Events[i].Cell) == cell {
			seg.Events[i].Index = scriptIndex
			found = true
			break
		}
	}
	if !found {
		seg.Events = append(seg.Events, events.Event{Cell: byte(cell), Index: scriptIndex})
	}
	m.Attr[cell] |= game.AttrHasEvent

	if !s.Key(ui.KeyForward) {
		t.Fatal("走進測試事件沒有作用")
	}
	if s.Mode != ui.ModeMessage || w.Pending == nil || w.Pending.Kind != game.PromptYesNo {
		t.Fatalf("Y/N 沒有立即出現：模式=%v，續跑=%+v", s.Mode, w.Pending)
	}
	if s.Message() != "Continue the test (y/n)?" {
		t.Errorf("提問文字是 %q", s.Message())
	}
	if got := w.Global(0); got != 0 {
		t.Errorf("尚未回答就跑進分支：全域值=%#x", got)
	}
	if !s.Key(ui.KeyYes) {
		t.Fatal("提問出現後的 Y 沒有作用")
	}
	if w.Pending != nil || s.Mode != ui.ModeExplore {
		t.Errorf("答完 Y 後是模式=%v、續跑=%+v", s.Mode, w.Pending)
	}
	if got := w.Global(0); got != 0x20 {
		t.Errorf("Y 分支全域值=%#x，預期 0x20", got)
	}
}

// 從新局用一般方向鍵走到中門西側，必須碰到**原始資料**裡的 Y/N 提問；
// 這不是塞一段測試腳本，而是 UI → 地圖 → EVENTSI → 暫停 → UI 的完整鏈。
func TestMiddlegateGateQuestionWaitsForAnswer(t *testing.T) {
	s := load(t)
	walkToMiddlegateGate(t, s)
	w := s.World()
	if s.Mode != ui.ModeMessage || w.Pending == nil || w.Pending.Kind != game.PromptYesNo {
		t.Fatalf("原始城門問題沒有停住：模式=%v，續跑=%+v，訊息=%q", s.Mode, w.Pending, w.Message)
	}
	if w.Pending.Segment != 61 || w.Pending.Script != 0 {
		t.Fatalf("中門特殊設施沒有轉到原版腳本庫 61/0：%+v", w.Pending)
	}
	if !strings.Contains(w.Message, "Sandsobar") {
		t.Errorf("原始腳本庫提問是 %q，預期提到 Sandsobar", w.Message)
	}
	if !strings.Contains(s.Message(), "桑德索巴") {
		t.Errorf("腳本庫提問沒有套用繁中譯文：%q", s.Message())
	}
	if s.Key(ui.KeyForward) {
		t.Error("原始 Y/N 提問掛著時，方向鍵不該被接受")
	}
	if w.X != 0 || w.Y != 5 {
		t.Errorf("問題未答時位置變成 (%d,%d)", w.X, w.Y)
	}
	if !s.Key(ui.KeyNo) {
		t.Fatal("原始城門問題的 N 沒有作用")
	}
	if w.Pending != nil || s.Mode != ui.ModeExplore {
		t.Errorf("答 N 後是模式=%v、續跑=%+v", s.Mode, w.Pending)
	}
	beforeMap, beforeX, beforeY, beforeFace := w.MapIndex, w.X, w.Y, w.Face
	hadWall, indoor := false, false
	if m := w.CurrentMap(); m != nil {
		hadWall, indoor = m.HasWall(w.X, w.Y, w.Face), m.Indoor
	}
	wantSolid := game.WallSolid.String()
	if !s.Key(ui.KeyForward) || s.Mode != ui.ModeMessage || s.Message() != wantSolid {
		t.Errorf("答 N 後向西應回原版的實牆訊息：前=圖%d (%d,%d) 面%v wall=%v indoor=%v；後=圖%d (%d,%d) 面%v，模式=%v、訊息=%q，預期=%q",
			beforeMap, beforeX, beforeY, beforeFace, hadWall, indoor,
			w.MapIndex, w.X, w.Y, w.Face, s.Mode, s.Message(), wantSolid)
	}
}

// 同一個原始問題的 Y 分支要付款並傳送；這條與 N 的輸入閘門測試合起來，
// 守住「顯示問題 → 收答案 → 只跑相應分支」的完整玩家路徑。
func TestMiddlegateGateYesTeleportsToSandsobar(t *testing.T) {
	s := load(t)
	walkToMiddlegateGate(t, s)
	w := s.World()
	beforeGold := 0
	for _, c := range s.Game.Party {
		beforeGold += c.Gold
	}
	if beforeGold < 10 {
		t.Skip("原版預設隊伍沒有足夠金幣驗付款分支")
	}
	if !s.Key(ui.KeyYes) {
		t.Fatal("原始城門問題的 Y 沒有作用")
	}
	if w.Pending != nil {
		t.Fatalf("答 Y 後仍停在輸入：%+v", w.Pending)
	}
	if w.MapIndex != 4 || w.X != 8 || w.Y != 1 {
		t.Fatalf("答 Y 後在圖%d (%d,%d)，預期 Sandsobar 圖4 (8,1)", w.MapIndex, w.X, w.Y)
	}
	afterGold := 0
	for _, c := range s.Game.Party {
		afterGold += c.Gold
	}
	if afterGold != beforeGold-10 {
		t.Errorf("答 Y 後金幣 %d，預期 %d", afterGold, beforeGold-10)
	}
}

func walkToMiddlegateGate(t *testing.T, s *ui.Session) {
	t.Helper()
	for i := 0; i < 2; i++ {
		if !s.Key(ui.KeyForward) {
			t.Fatalf("北走第 %d 步沒有作用", i+1)
		}
		if s.Mode != ui.ModeExplore {
			t.Fatalf("北走第 %d 步不該被招牌攔住，實際是 %v", i+1, s.Mode)
		}
	}
	for i := 0; i < 3; i++ {
		s.Key(ui.KeyRight)
	}
	for i := 0; i < 7; i++ {
		if !s.Key(ui.KeyForward) {
			t.Fatalf("西走第 %d 步沒有作用", i+1)
		}
	}
	w := s.World()
	if w.MapIndex != 0 || w.X != 0 || w.Y != 5 {
		t.Fatalf("到達的位置是圖%d (%d,%d)，預期 Middlegate (0,5)", w.MapIndex, w.X, w.Y)
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

// 需要輸入的法術必須先進入可取消子選單；Esc 不能在提示尚未確認時扣費。
// 這條走正常的 C → 施法者 → 法術 → 隊員提示鍵路徑，不直接呼叫 Cast。
func TestTargetedSpellPromptCanCancelBeforeCost(t *testing.T) {
	s := load(t)
	who := -1
	for i := range s.Game.Party {
		if s.Game.Party[i].Class == game.Cleric && s.Game.Party[i].Condition.Acts() {
			who = i
			break
		}
	}
	if who < 0 {
		t.Skip("隊伍裡沒有可用牧師")
	}
	// 急救術是已解出的隊員 consumer；讓測試只依賴正常學法術資料流。
	s.Game.Party[who].Learn(4)
	before := s.Game.Party[who].SP
	if !s.Key(ui.KeyCast) {
		t.Fatal("無法開啟施法選單")
	}
	for i := 0; i < len(s.Menu.Items) && !strings.HasPrefix(s.Menu.Items[s.Menu.Cur], s.Game.Party[who].Name); i++ {
		s.Key(ui.KeyDown)
	}
	if !s.Key(ui.KeyConfirm) || s.Mode != ui.ModeMenu {
		t.Fatal("無法進入法術清單")
	}
	for i := 0; i < len(s.Menu.Items) && !strings.Contains(s.Menu.Items[s.Menu.Cur], "急救術"); i++ {
		s.Key(ui.KeyDown)
	}
	if !strings.Contains(s.Menu.Items[s.Menu.Cur], "急救術") {
		t.Fatal("法術清單沒有急救術")
	}
	if !s.Key(ui.KeyConfirm) || s.Mode != ui.ModeMenu || s.Game.Party[who].SP != before {
		t.Fatal("選擇隊員目標前不應扣法力")
	}
	lineCount := len(s.Lines)
	if s.Key(ui.KeySave) || len(s.Lines) != lineCount || s.Game.Party[who].SP != before {
		t.Fatal("提示中的存檔鍵不應改變狀態；提示不是可存檔狀態")
	}
	if !s.Key(ui.KeyCancel) || s.Mode != ui.ModeMenu || s.Game.Party[who].SP != before {
		t.Fatal("Esc 沒有取消目標提示或提前扣費")
	}
	if !strings.Contains(s.Menu.Title, "要施什麼法術") {
		t.Fatalf("取消後沒有回到法術清單：%q", s.Menu.Title)
	}
	// 再次選擇同一法術，確認隊員後才扣費。
	for i := 0; i < len(s.Menu.Items) && !strings.Contains(s.Menu.Items[s.Menu.Cur], "急救術"); i++ {
		s.Key(ui.KeyDown)
	}
	s.Key(ui.KeyConfirm)
	if s.Menu == nil || !strings.Contains(s.Menu.Title, "請選一名隊員") {
		t.Fatal("沒有開啟隊員目標提示")
	}
	s.Key(ui.KeyConfirm)
	if s.Game.Party[who].SP >= before {
		t.Fatal("確認隊員後沒有扣法力")
	}
}

// 物品與數字提示也必須在正常 UI 中先停住；Esc 返回法術清單且不扣費。
func TestSpellItemAndChoicePromptsCanCancel(t *testing.T) {
	s := load(t)
	// 能量補充術（engine index 82）是巫師系第 35 條，明確讀背包槽位。
	itemWho := -1
	for i := range s.Game.Party {
		if s.Game.Party[i].Class == game.Sorcerer && s.Game.Party[i].Condition.Acts() {
			itemWho = i
			break
		}
	}
	if itemWho < 0 {
		t.Skip("隊伍裡沒有可用巫師")
	}
	s.Game.Party[itemWho].Learn(35)
	s.Game.Party[itemWho].Learn(31)
	s.Game.Party[itemWho].Items[game.EquippedSlots] = game.ItemSlot{ID: 1, Charge: 3}
	before := s.Game.Party[itemWho].SP
	if !s.Key(ui.KeyCast) {
		t.Fatal("無法開啟施法選單")
	}
	for i := 0; i < len(s.Menu.Items) && !strings.HasPrefix(s.Menu.Items[s.Menu.Cur], s.Game.Party[itemWho].Name); i++ {
		s.Key(ui.KeyDown)
	}
	s.Key(ui.KeyConfirm)
	for i := 0; i < len(s.Menu.Items) && !strings.Contains(s.Menu.Items[s.Menu.Cur], "能量補充術"); i++ {
		s.Key(ui.KeyDown)
	}
	if !strings.Contains(s.Menu.Items[s.Menu.Cur], "能量補充術") {
		t.Skip("資料未提供能量補充術名稱")
	}
	s.Key(ui.KeyConfirm)
	if !s.Key(ui.KeyCancel) || s.Game.Party[itemWho].SP != before {
		t.Fatal("物品提示取消失敗或提前扣費")
	}

	// 傳送術（engine index 78）是巫師系第 31 條，明確讀 1–9。
	for i := 0; i < len(s.Menu.Items) && !strings.Contains(s.Menu.Items[s.Menu.Cur], "傳送術"); i++ {
		s.Key(ui.KeyDown)
	}
	if !strings.Contains(s.Menu.Items[s.Menu.Cur], "傳送術") {
		t.Skip("資料未提供傳送術名稱")
	}
	s.Key(ui.KeyConfirm)
	if s.Menu == nil || !strings.Contains(s.Menu.Title, "數字") {
		t.Fatal("沒有開啟數字提示")
	}
	if !s.Key(ui.KeyCancel) || s.Game.Party[itemWho].SP != before {
		t.Fatal("數字提示取消失敗或提前扣費")
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
		t.Fatalf("在地圖 %d 開鐵匠鋪失敗（模式 %v）", s.World().MapIndex, s.Mode)
	}
	// 鐵匠鋪的主選單第一項是「購買」，選進去才是貨架。
	s.Key(ui.KeyConfirm)
	if s.Mode != ui.ModeMenu || s.Menu == nil {
		t.Fatal("進不了貨架")
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
			drainEventInputUI(t, s)
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

// drainEventInputUI 只讓傳送路徑測試走過原版事件的輸入門檻。它刻意把
// Y/N 放在**踩到事件之後**才送入，避免測試重新引入「先設定下一題答案」
// 那個舊介面。
func drainEventInputUI(t *testing.T, s *ui.Session) {
	t.Helper()
	for n := 0; s.World().Pending != nil; n++ {
		if n >= 32 {
			t.Fatalf("事件輸入連續停了超過 32 次：%+v", s.World().Pending)
		}
		switch s.World().Pending.Kind {
		case game.PromptKey:
			if !s.Key(ui.KeyConfirm) {
				t.Fatal("0x07 的確認鍵沒有作用")
			}
		case game.PromptYesNo:
			if !s.Key(ui.KeyYes) {
				t.Fatal("事件出現後的 Y 沒有作用")
			}
		case game.PromptMember:
			if !s.Key(ui.KeyCancel) {
				t.Fatal("選人事件無法取消")
			}
		case game.PromptText:
			for _, r := range s.World().TextExpect {
				s.TypeRune(r)
			}
			if !s.Key(ui.KeyConfirm) {
				t.Fatal("文字事件無法送出答案")
			}
		default:
			t.Fatalf("未知事件輸入種類 %q", s.World().Pending.Kind)
		}
	}
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
	if len(s.Menu.Items) != 9 {
		t.Errorf("第一層有 %d 類，預期 9（六類表格 + 序言 + 科隆的歷史 + 謎題答案）",
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
	if len(s.Menu.Items) != 9 {
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
	}{{ui.KeyBash, "撞門"}} {
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

// 開鎖要先挑人再動手 —— 盜行最高的那一個可能中毒或不在。
func TestUnlockPicksMember(t *testing.T) {
	s := load(t)
	s.Lines = nil
	s.Mode = ui.ModeExplore
	if !s.Key(ui.KeyUnlock) || s.Mode != ui.ModeMenu {
		t.Fatalf("開鎖沒有進選人選單，現在是 %v", s.Mode)
	}
	lines := s.Draw() // 畫得出來就好，不看內容
	_ = lines
	if !s.Key(ui.KeyConfirm) {
		t.Fatal("確認鍵沒有回報變化")
	}
	if len(s.Lines) == 0 {
		t.Fatal("開鎖沒有任何訊息")
	}
	t.Logf("開鎖：%s", s.Lines[0])
	if s.Mode != ui.ModeMessage {
		t.Errorf("開鎖之後是 %v，預期訊息模式", s.Mode)
	}

	// 取消要回到探索，而且什麼都不做
	s.Lines = nil
	s.Mode = ui.ModeExplore
	s.Key(ui.KeyUnlock)
	s.Key(ui.KeyCancel)
	if s.Mode == ui.ModeMenu {
		t.Error("取消之後還留在選單裡")
	}
	if len(s.Lines) != 0 {
		t.Errorf("取消卻還是開了鎖：%v", s.Lines)
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

// 正常戰鬥回合獲勝後，戰利品擺著等玩家按 `S`——原版 `2COMBAT sub_19BF8`
// 只把 `ds:0434` 清成 0，四選單要 root `0x13814`（Search）才會出現。
func TestCombatVictoryNeedsSearch(t *testing.T) {
	s := load(t)
	var d monsters.Monster
	d.Index, d.HP, d.SpecialUses, d.Speed, d.AC = 0x21, 1, 1, 1, 1
	d.DropBand, d.GoldMode, d.GemDrop = 1, 1, true
	m := game.NewMonster(d)
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	s.Game.Fight = &game.Encounter{Party: party, Monsters: []game.Combatant{m}, Front: 1}
	s.Mode = ui.ModeCombat
	for i := 0; i < 20 && s.Mode == ui.ModeCombat; i++ {
		if !s.Key(ui.KeyConfirm) {
			t.Fatal("戰鬥確認鍵沒有推進回合")
		}
	}
	if s.Mode != ui.ModeMessage {
		t.Fatalf("勝利後模式是 %v，預期停在訊息", s.Mode)
	}
	if s.Chest == nil {
		t.Fatal("勝利後沒有留下戰利品")
	}
	if !strings.Contains(strings.Join(s.Lines, "|"), "搜尋") {
		t.Errorf("沒有提示玩家搜尋：%v", s.Lines)
	}
	for i := 0; i < 10 && s.Mode == ui.ModeMessage; i++ {
		s.Key(ui.KeyConfirm) // 讀完戰報回到探索
	}
	if !s.Key(ui.KeySearch) || s.Mode != ui.ModeMenu {
		t.Fatalf("按 S 沒有進寶箱選單，現在是 %v", s.Mode)
	}
	if s.Chest == nil || len(s.Menu.Items) != 4 {
		t.Fatalf("勝利後沒有一般寶箱四選單：箱子=%v 選項=%v", s.Chest, s.Menu.Items)
	}
	if s.Menu.Items[0] != "打開" || s.Menu.Items[3] != "離開" {
		t.Errorf("寶箱選項不對：%v", s.Menu.Items)
	}
	// 離開箱子會清除 UI 狀態，不會把戰鬥中的箱子留到下一次探索。
	s.Key(ui.KeyDown)
	s.Key(ui.KeyDown)
	s.Key(ui.KeyDown)
	s.Key(ui.KeyConfirm)
	if s.Chest != nil {
		t.Fatal("離開寶箱後 Chest 仍存在")
	}
}

// 戰鬥中施法之後要回到戰鬥，不是回到探索。
//
// 這個 bug 的症狀是「施完法就從戰鬥裡走出來了」，而且不會報錯。
func TestCastInCombatStaysInCombat(t *testing.T) {
	s := load(t)
	var d monsters.Monster
	d.HP, d.SpecialUses, d.Speed, d.AC = 50, 1, 1, 1
	m := game.NewMonster(d)
	m.Display = "測試怪"
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	s.Game.Fight = &game.Encounter{Party: party, Monsters: []game.Combatant{m}}
	s.Mode = ui.ModeCombat

	if !s.Key(ui.KeyCast) || s.Mode != ui.ModeMenu {
		t.Fatalf("戰鬥中按 C 之後是 %v，預期開選單", s.Mode)
	}
	// 挑到會法術的人再施一條
	for i := 0; i < len(s.Menu.Items); i++ {
		if strings.HasPrefix(s.Menu.Items[s.Menu.Cur], "Gene Eric") {
			break
		}
		s.Key(ui.KeyDown)
	}
	s.Key(ui.KeyConfirm)
	s.Key(ui.KeyConfirm)
	if s.Mode != ui.ModeCombat {
		t.Errorf("施完法之後是 %v，預期回到戰鬥", s.Mode)
	}
	if s.Game.Fight == nil {
		t.Error("戰鬥不見了")
	}
}

// 射擊指令要打得到前排以外的敵人。
//
// 這一場的前排刻意設 0：近戰整隊白站，射擊照樣打得到 —— 兩者的差別
// 就在這裡，不是命中率。
func TestCombatShootReachesBackRank(t *testing.T) {
	s := load(t)
	var d monsters.Monster
	d.HP, d.SpecialUses, d.Speed, d.AC = 60, 1, 1, 1
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	newFight := func() *game.Encounter {
		m := game.NewMonster(d)
		m.Display = "後排怪"
		return &game.Encounter{Party: party, Monsters: []game.Combatant{m}, Front: 0}
	}

	s.Game.Fight = newFight()
	s.Mode = ui.ModeCombat
	hp := s.Game.Fight.Monsters[0].CombatHP()
	s.Key(ui.KeyConfirm) // 近戰
	if got := s.Game.Fight.Monsters[0].CombatHP(); got != hp {
		t.Errorf("前排 0 卻被近戰打到：%d → %d", hp, got)
	}

	s.Game.Fight = newFight()
	s.Mode = ui.ModeCombat
	hp = s.Game.Fight.Monsters[0].CombatHP()
	hit := false
	for i := 0; i < 20 && s.Game.Fight != nil && !hit; i++ {
		s.Key(ui.KeyShoot)
		if s.Game.Fight != nil && s.Game.Fight.Monsters[0].CombatHP() < hp {
			hit = true
		}
	}
	if !hit {
		t.Errorf("射擊二十回合都沒碰到後排（HP 仍是 %d）", hp)
	}
	// 打完要放掉旗標，下一回合回到近戰
	if s.Game.Fight != nil && s.Game.Fight.Ranged {
		t.Error("射擊旗標沒放掉，下一回合會繼續走射擊")
	}

	// 留一張畫面：播報要看得到中文的攻擊動詞。
	out := filepath.Join("workplace", "gfx", "ui")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(out, "combat-shoot.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, s.Draw().Hi); err != nil {
		t.Fatal(err)
	}
	t.Logf("播報：%v", s.Lines)
}

// 物品選單裡按「使用」要真的發動附帶的法術並扣充能。
func TestUseItemFromMenu(t *testing.T) {
	s := load(t)
	// 找一件法術型的東西塞進第一個人的背包
	id := -1
	for i, it := range s.Game.Items {
		if _, ok := it.UseSpell(); ok {
			id = i
			break
		}
	}
	if id < 0 {
		t.Fatal("物品表裡沒有法術型的東西")
	}
	who := 0
	slot := game.EquippedSlots
	s.Game.Party[who].Items[slot] = game.ItemSlot{ID: id, Charge: 3}

	if !s.Key(ui.KeyItems) || s.Mode != ui.ModeMenu {
		t.Fatal("開不了物品選單")
	}
	for i := 0; i < slot; i++ {
		s.Key(ui.KeyDown)
	}
	before := len(s.Lines)
	if !s.Key(ui.KeyUse) {
		t.Fatal("使用鍵沒有回報變化")
	}
	if len(s.Lines) == before {
		t.Fatal("使用之後沒有任何播報")
	}
	line := s.Lines[len(s.Lines)-1]
	t.Logf("播報：%s", line)
	if got := s.Game.Party[who].Items[slot].Charge; got != 2 {
		t.Errorf("充能剩 %d，預期 2", got)
	}
	if !strings.Contains(line, "發動") {
		t.Errorf("播報沒說發動了什麼：%s", line)
	}
}

// 非魔法物品必須顯示實際改變，不能把預設 Spell=0 誤播成施法。
func TestUseNonSpellItemFromMenu(t *testing.T) {
	s := load(t)
	id := -1
	for i, it := range s.Game.Items {
		if it.Use == 0x1A {
			id = i
			break
		}
	}
	if id < 0 {
		t.Fatal("物品表裡沒有 Force Potion 類效果")
	}
	who := 0
	slot := game.EquippedSlots
	s.Game.Party[who].Current[game.Might] = 15
	s.Game.Party[who].Items[slot] = game.ItemSlot{ID: id, Charge: 2, Attr: 3}

	if !s.Key(ui.KeyItems) || s.Mode != ui.ModeMenu {
		t.Fatal("開不了物品選單")
	}
	for i := 0; i < slot; i++ {
		s.Key(ui.KeyDown)
	}
	if !s.Key(ui.KeyUse) {
		t.Fatal("使用鍵沒有回報變化")
	}
	line := s.Lines[len(s.Lines)-1]
	if strings.Contains(line, "發動") {
		t.Errorf("非魔法物品卻播成施法：%s", line)
	}
	if !strings.Contains(line, "力量 15 → 28") {
		t.Errorf("非魔法效果播報不完整：%s", line)
	}
	if got := s.Game.Party[who].Current[game.Might]; got != 28 {
		t.Errorf("介面使用後力量是 %d，預期 28", got)
	}
}

// 戰鬥中的 P 與 V 是純顯示指令，要有東西可看而且不推進回合。
func TestCombatProtectAndView(t *testing.T) {
	s := load(t)
	var d monsters.Monster
	d.HP, d.SpecialUses, d.Speed, d.AC = 50, 1, 1, 1
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	m := game.NewMonster(d)
	s.Game.Fight = &game.Encounter{
		Party: party, Monsters: []game.Combatant{m}, Front: 1,
		Protect: game.Protection{Bless: 2, HolyBonus: 9},
	}
	s.Mode = ui.ModeCombat
	hp := m.CombatHP()

	if !s.Key(ui.KeyProt) {
		t.Fatal("P 沒有回報變化")
	}
	line := s.Lines[len(s.Lines)-1]
	if !strings.Contains(line, "祝福術") || !strings.Contains(line, "聖光加值 9") {
		t.Errorf("防護畫面內容不對：%s", line)
	}

	if !s.Key(ui.KeyView) {
		t.Fatal("V 沒有回報變化")
	}
	joined := strings.Join(s.Lines, "|")
	if !strings.Contains(joined, "生命") || !strings.Contains(joined, "揮擊") {
		t.Errorf("檢視畫面內容不對：%v", s.Lines)
	}
	// 兩個都是純顯示，不該打一回合
	if m.CombatHP() != hp {
		t.Errorf("純顯示指令卻推進了戰鬥：%d → %d", hp, m.CombatHP())
	}
}

// 對調換的是戰鬥隊形（誰站前面誰先挨打），不是名冊順序。
func TestCombatExchange(t *testing.T) {
	s := load(t)
	var d monsters.Monster
	d.HP, d.SpecialUses, d.Speed, d.AC = 99, 1, 1, 1
	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	rosterFirst := s.Game.Party[0].Name
	s.Game.Fight = &game.Encounter{
		Party: party, Monsters: []game.Combatant{game.NewMonster(d)}, Front: 1,
	}
	s.Mode = ui.ModeCombat
	before := s.Game.Fight.Party[0].CombatName()

	if !s.Key(ui.KeyExch) || s.Mode != ui.ModeMenu {
		t.Fatalf("對調沒有進選單，現在是 %v", s.Mode)
	}
	s.Key(ui.KeyConfirm) // 選第 1 位
	if s.Mode != ui.ModeMenu {
		t.Fatal("選完第一位就離開選單了，第二位沒得選")
	}
	s.Key(ui.KeyDown) // 移到第 2 位
	s.Key(ui.KeyDown)
	s.Key(ui.KeyConfirm)

	if s.Mode != ui.ModeCombat {
		t.Errorf("對調完是 %v，預期回到戰鬥", s.Mode)
	}
	after := s.Game.Fight.Party[0].CombatName()
	if after == before {
		t.Errorf("隊形沒有變，第一位還是 %s", before)
	}
	t.Logf("第一位 %s → %s；播報：%s", before, after, s.Lines[len(s.Lines)-1])
	if s.Game.Party[0].Name != rosterFirst {
		t.Errorf("名冊順序被動到了：%s → %s", rosterFirst, s.Game.Party[0].Name)
	}
}

// 地圖畫面：城鎮整張看得到（手冊本來就印了），其他地圖只顯示走過的格。
func TestMapView(t *testing.T) {
	s := load(t)
	if !s.Key(ui.KeyMap) || s.Mode != ui.ModeMap {
		t.Fatalf("M 沒有開地圖，現在是 %v", s.Mode)
	}
	town := s.Draw()
	townPix := make([]byte, len(town.Orig.Pix))
	copy(townPix, town.Orig.Pix)

	out := filepath.Join("workplace", "gfx", "ui")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(out, "map-town.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, town.Hi); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	// 任意鍵離開
	if !s.Key(ui.KeyConfirm) || s.Mode != ui.ModeExplore {
		t.Errorf("離開地圖之後是 %v", s.Mode)
	}

	// 換到一張沒走過的非城鎮地圖：畫面應該幾乎全黑（只有外框與隊伍）
	dark := -1
	for i := 0; i < len(s.Game.World.Maps); i++ {
		if !game.Mapped(i) {
			dark = i
			break
		}
	}
	if dark < 0 {
		t.Skip("沒有非城鎮的地圖")
	}
	s.Game.World.MapIndex = dark
	s.Game.World.Explored = game.Explored{}
	s.Mode = ui.ModeMap
	unknown := s.Draw()
	if equalBytes(townPix, unknown.Orig.Pix) {
		t.Error("沒走過的地圖畫得跟城鎮一樣")
	}
	lit := 0
	for _, p := range unknown.Orig.Pix {
		if p != 0 {
			lit++
		}
	}
	total := len(unknown.Orig.Pix)
	if lit*100/total > 15 {
		t.Errorf("沒走過的地圖亮了 %d%% 的像素，等於把地城攤開了", lit*100/total)
	}
	t.Logf("城鎮全圖已輸出；未探索地圖只亮 %d%%", lit*100/total)
}

// 戰鬥中畫面上要看得到怪物。
//
// 這個 bug 的形狀是「怪物畫進了原版像素層，但那一層已經 Flush 過了」——
// 測試全綠、也沒有任何錯誤，只是畫面上一隻怪都沒有。所以要量高解析層。
func TestCombatDrawsMonster(t *testing.T) {
	s := load(t)
	if len(s.Game.Bestiary) < 4 {
		t.Skip("沒有圖鑑")
	}
	base := s.Draw()
	before := snapshot(base.Hi)

	party := make([]game.Combatant, 0, len(s.Game.Party))
	for i := range s.Game.Party {
		party = append(party, &s.Game.Party[i])
	}
	m := game.NewMonster(s.Game.Bestiary[3])
	s.Game.Fight = &game.Encounter{
		Party: party, Monsters: []game.Combatant{m}, Front: 1,
	}
	s.Mode = ui.ModeCombat
	after := snapshot(s.Draw().Hi)

	// 數的是**變掉的**像素，不是亮起來的 —— 怪物站在牆前面，
	// 牠的暗色輪廓會把原本亮的牆蓋掉，只數亮點會得到負數。
	diff := 0
	for i := range before {
		if before[i] != after[i] {
			diff++
		}
	}
	if diff < 2000 {
		t.Errorf("開打之後畫面只變了 %d 個像素，怪物沒畫出來", diff)
	}
	t.Logf("怪物讓畫面變了 %d 個像素", diff)
}

func snapshot(im *image.RGBA) []byte {
	out := make([]byte, len(im.Pix))
	copy(out, im.Pix)
	return out
}

// 寶箱那一頁：四個選項，選了動作再挑人。
func TestChestPage(t *testing.T) {
	s := load(t)
	s.Chest = &game.Chest{Gold: 300, Trap: 0}
	gold := s.Game.Party[0].Gold

	if !s.Key(ui.KeySearch) || s.Mode != ui.ModeMenu {
		t.Fatalf("開不了寶箱那一頁，現在是 %v", s.Mode)
	}
	s.Key(ui.KeyConfirm) // 1) 打開
	if s.Mode != ui.ModeMenu {
		t.Fatal("選完動作沒有跳出挑人的選單")
	}
	s.Key(ui.KeyConfirm) // 第一位
	if s.Chest != nil {
		t.Error("開完之後箱子還在")
	}
	if s.Game.Party[0].Gold != gold+300 {
		t.Errorf("金幣 %d → %d，預期加 300", gold, s.Game.Party[0].Gold)
	}
	joined := strings.Join(s.Lines, "|")
	if !strings.Contains(joined, "300") {
		t.Errorf("播報裡沒有金幣：%v", s.Lines)
	}
	t.Logf("播報：%v", s.Lines)

	// 沒有東西可撿時，對應原版的 `Nothing Here!`
	s.Lines = nil
	s.Mode = ui.ModeExplore
	s.Key(ui.KeySearch)
	if s.Mode == ui.ModeMenu {
		t.Error("沒有箱子卻進了選單")
	}
	if len(s.Lines) == 0 || !strings.Contains(s.Lines[0], "什麼都沒有") {
		t.Errorf("沒有箱子時的回應：%v", s.Lines)
	}
}

// 建角色的完整流程：擲屬性 → 對調 → 選職業 → 種族 → 陣營 → 性別 → 命名。
func TestCreateCharacterFlow(t *testing.T) {
	s := load(t)
	if !s.Key(ui.KeyCreate) || s.Mode != ui.ModeCreate {
		t.Fatalf("進不了建角畫面，現在是 %v", s.Mode)
	}
	// 擲到有職業可選為止（Enter 重擲）
	pick := -1
	for i := 0; i < 200 && pick < 0; i++ {
		for c := 0; c < 8; c++ {
			if game.EligibleClasses(s.New.Attr)[c] {
				pick = c + 1
				break
			}
		}
		if pick < 0 {
			s.Key(ui.KeyConfirm)
		}
	}
	if pick < 0 {
		t.Fatal("擲了兩百次都沒有可選的職業")
	}
	t.Logf("屬性 %v，選第 %d 個職業", s.New.Attr, pick)

	if !s.PressDigit(pick) || s.Mode != ui.ModeMenu {
		t.Fatalf("選完職業沒有進種族選單，現在是 %v", s.Mode)
	}
	s.Key(ui.KeyConfirm) // 種族
	s.Key(ui.KeyConfirm) // 陣營
	s.Key(ui.KeyConfirm) // 性別
	if s.Mode != ui.ModeName {
		t.Fatalf("選完性別沒有進命名，現在是 %v", s.Mode)
	}

	// 空名字不能存
	before := len(s.Game.Roster)
	s.Key(ui.KeyConfirm)
	if len(s.Game.Roster) != before {
		t.Error("空名字也存進名冊了")
	}
	for _, r := range "阿光" {
		s.TypeRune(r)
	}
	s.TypeRune('\b')
	s.TypeRune('光')
	if s.New.Name != "阿光" {
		t.Errorf("名字是 %q，預期「阿光」", s.New.Name)
	}
	if !s.Key(ui.KeyConfirm) {
		t.Fatal("存檔沒有回報變化")
	}
	if len(s.Game.Roster) != before+1 {
		t.Fatalf("名冊沒有增加：%d → %d", before, len(s.Game.Roster))
	}
	c := s.Game.Roster[len(s.Game.Roster)-1]
	if c.Name != "阿光" || c.Level != 1 || c.HP < 1 {
		t.Errorf("新角色不對：%+v", c)
	}
	t.Logf("建好 %s：%v %v，生命 %d", c.Name, c.Race, c.Class, c.HP)

	// 名字上限是 10 個字
	s.Mode = ui.ModeName
	s.New.Name = ""
	for i := 0; i < 20; i++ {
		s.TypeRune('A')
	}
	if n := len([]rune(s.New.Name)); n != 10 {
		t.Errorf("名字長度 %d，記錄的名字欄只有 10 bytes", n)
	}
}

// 屬性對調要真的換，而且可選職業跟著變。
func TestCreateExchangeAttrs(t *testing.T) {
	s := load(t)
	s.Key(ui.KeyCreate)
	before := s.New.Attr
	s.Key(ui.KeyUse)  // 挑起第一項
	s.Key(ui.KeyDown) // 游標移到第二項
	s.Key(ui.KeyConfirm)
	if s.New.Attr[0] != before[1] || s.New.Attr[1] != before[0] {
		t.Errorf("對調沒生效：%v → %v", before, s.New.Attr)
	}
}

// 旅店是名冊與隊伍的編組畫面：編入、移出、隊伍上限。
func TestInnPartyManagement(t *testing.T) {
	s := load(t)
	// 名冊放兩個人
	extra := s.Game.Party[0]
	extra.Name = "候補甲"
	s.Game.Roster = append(s.Game.Roster, extra)
	extra.Name = "候補乙"
	s.Game.Roster = append(s.Game.Roster, extra)

	// 直接開旅店選單（踩進設施那條路徑由 step 測）
	s.Game.Facility = game.FacilityInn
	s.Key(ui.KeyForward)
	if s.Mode != ui.ModeMenu {
		// 沒踩到設施格也沒關係，直接驗引擎那一層
		s.Mode = ui.ModeExplore
	}
	// 隊伍原本就滿六人，編入要被擋
	if msg, ok := s.Game.AddToParty(0); ok || !strings.Contains(msg, "已滿") {
		t.Errorf("六人滿隊還編得進去：%q", msg)
	}

	// 移出一個再編入
	before := len(s.Game.Party)
	if _, ok := s.Game.RemoveFromParty(0); !ok {
		t.Fatal("移不出隊伍")
	}
	if len(s.Game.Party) != before-1 {
		t.Fatalf("移出之後 %d 人", len(s.Game.Party))
	}
	msg, ok := s.Game.AddToParty(0)
	if !ok {
		t.Fatalf("空出位置還編不進去：%s", msg)
	}
	if len(s.Game.Party) != before {
		t.Errorf("編入之後 %d 人，預期 %d", len(s.Game.Party), before)
	}
	// 同一個人不能編兩次
	if _, ok := s.Game.AddToParty(0); ok {
		t.Error("同一個人編了兩次")
	}
}

// 鐵匠鋪的出售與鑑定。
func TestSmithSellAndIdentify(t *testing.T) {
	s := load(t)
	c := &s.Game.Party[0]
	// 背包放一件有價格的東西
	id := 0
	for i, it := range s.Game.Items {
		if i > 0 && it.Price > 0 {
			id = i
			break
		}
	}
	if id == 0 {
		t.Skip("物品表裡沒有有價格的東西")
	}
	c.Items[game.EquippedSlots] = game.ItemSlot{ID: id, Charge: 3}
	c.Gold = 5000
	gold := c.Gold

	lines, ok := s.Game.IdentifyItem(0, 0)
	if !ok {
		t.Fatalf("鑑定失敗：%v", lines)
	}
	if c.Gold >= gold {
		t.Errorf("鑑定沒收錢（%d → %d）", gold, c.Gold)
	}
	t.Logf("鑑定：%v", lines)

	gold = c.Gold
	msg, ok := s.Game.SellItem(0, 0)
	if !ok {
		t.Fatalf("出售失敗：%s", msg)
	}
	if c.Gold <= gold {
		t.Errorf("賣完金幣沒增加（%d → %d）", gold, c.Gold)
	}
	if !c.Backpack()[0].Empty() {
		t.Error("賣完那一格還有東西")
	}
	t.Logf("出售：%s", msg)

	// 空格不能賣
	if _, ok := s.Game.SellItem(0, 0); ok {
		t.Error("空格也賣得掉")
	}
}

// 說明書的敘事章節（序言與科隆的歷史）也要查得到。
//
// 那兩章只印在紙本上，遊戲裡一個字都沒有 —— 而故事的來龍去脈全在那裡。
func TestReferenceLore(t *testing.T) {
	s := load(t)
	if s.Ref == nil {
		t.Skip("沒有 data/reference.json")
	}
	s.Key(ui.KeyRef)
	n := len(s.Menu.Items)
	if n < 7 {
		t.Fatalf("第一層只有 %d 類", n)
	}
	if last := s.Menu.Items[n-1]; !strings.Contains(last, "謎題") {
		t.Errorf("最後一類是 %q，預期打字謎題的答案", last)
	}
	// 科隆的歷史排在謎題前面一格
	for i := 0; i < n-2; i++ {
		s.Key(ui.KeyDown)
	}
	if cur := s.Menu.Items[s.Menu.Cur]; !strings.Contains(cur, "科隆的歷史") {
		t.Fatalf("倒數第二類是 %q，預期科隆的歷史", cur)
	}
	s.Key(ui.KeyConfirm)
	if len(s.Menu.Items) < 20 {
		t.Fatalf("科隆的歷史只有 %d 段", len(s.Menu.Items))
	}
	joined := strings.Join(s.Menu.Items, "|")
	for _, want := range []string{"卡隆", "火龍", "水晶球"} {
		if !strings.Contains(joined, want) {
			t.Errorf("內容裡找不到「%s」", want)
		}
	}
	t.Logf("科隆的歷史共 %d 段，第一段：%s", len(s.Menu.Items), s.Menu.Items[0])
}

// 物品名與怪物名的譯文鍵是**編號**（`item.%03d`／`monster.%03d`）。
// 拿原文當鍵去查，一條都查不到，而查不到的行為就是顯示原文 ——
// 整批 256 個名字沒接上，畫面看起來只是「還沒翻」，不會有任何錯誤。
func TestItemAndMonsterNamesAreTranslated(t *testing.T) {
	s := load(t)
	hasCJK := func(v string) bool {
		for _, r := range v {
			if r > 0x7F {
				return true
			}
		}
		return false
	}

	n, en := 0, []string{}
	for _, it := range s.Game.Items {
		if it.Name == "" {
			continue
		}
		n++
		if !hasCJK(it.Name) {
			en = append(en, it.Name)
		}
	}
	if n == 0 {
		t.Fatal("物品表是空的")
	}
	if len(en) > 0 {
		t.Errorf("%d／%d 個物品名沒接上譯文，例如 %q", len(en), n, en[:min(5, len(en))])
	}

	if len(s.Game.Names) == 0 {
		t.Fatal("怪物名一條譯文都沒接上")
	}
	for src, dst := range s.Game.Names {
		if !hasCJK(dst) {
			t.Errorf("怪物 %q 的譯文 %q 不是中文", src, dst)
			break
		}
	}
}

// 譯文用到的字都要在 atlas 裡。缺字不會報錯，只會畫成空白 ——
// 「昆登」少一個字就變成「　登」，讀的人不知道少了什麼。
func TestTranslatedNamesHaveGlyphs(t *testing.T) {
	s := load(t)
	f := s.Assets.CJK
	if f == nil {
		t.Skip("沒有中文 atlas")
	}
	var miss []rune
	seen := map[rune]bool{}
	add := func(v string) {
		for _, r := range f.Missing(v) {
			if !seen[r] {
				seen[r] = true
				miss = append(miss, r)
			}
		}
	}
	for _, it := range s.Game.Items {
		add(it.Name)
	}
	for _, v := range s.Game.Names {
		add(v)
	}
	if len(miss) > 0 {
		t.Errorf("atlas 缺 %d 個字：%q（重跑 tools/build_cjk_font.py）", len(miss), string(miss))
	}
}

// 選單一行放不下就會被折成兩行，畫面上看起來像「價格掉到下一行」。
// 折行本身不會報錯，所以要在這裡守住寬度。
func TestMenuLinesFitBox(t *testing.T) {
	s := load(t)
	limit := view.MenuCols()
	opens := []struct {
		name string
		keys []ui.Key
	}{
		{"物品", []ui.Key{ui.KeyItems}},
		{"商店", []ui.Key{ui.KeyShop, ui.KeyConfirm}},
		{"施法", []ui.Key{ui.KeyCast, ui.KeyDown, ui.KeyDown, ui.KeyDown, ui.KeyConfirm}},
		{"查說明書", []ui.Key{ui.KeyRef}},
	}
	for _, o := range opens {
		s.Key(ui.KeyCancel)
		s.Key(ui.KeyCancel)
		for _, k := range o.keys {
			s.Key(k)
		}
		if s.Menu == nil {
			t.Errorf("%s 選單沒開起來", o.name)
			continue
		}
		for _, l := range s.Menu.Lines() {
			if n := len([]rune(l)); n > limit {
				t.Errorf("%s 的一行 %d 個字，超過 %d：%q", o.name, n, limit, l)
			}
		}
	}
}

// 打字謎題的答案要出現在提問後面，也要查得到。
//
// 原版的謎底靠英文文字遊戲（`What has Mark lost?` → `KEYS`），
// 翻成中文之後線索與答案對不起來 —— 不附答案就永遠解不開。
func TestPuzzleAnswerShown(t *testing.T) {
	s := load(t)
	// 直接跑那段腳本：EVENTSO 段 9 的 Death Spider 謎語。
	w := s.Game.World
	w.TextExpect = ""
	w.RunScriptForTest([]byte{0x2f, 0x30,
		0xcf, 0xd5, 0xc1, 0xc7, 0xfa, 0xfa, 0xfa, 0xfa, 0xfa, 0xfa})
	if w.TextExpect != "KEYS" {
		t.Fatalf("解出來的答案是 %q，預期 KEYS", w.TextExpect)
	}
	s.Game.Log = append(s.Game.Log, "unless you answer this riddle.")
	s.Take()
	joined := strings.Join(s.Lines, "|")
	if !strings.Contains(joined, "KEYS") {
		t.Errorf("答案沒有附在訊息後面：%q", s.Lines)
	}

	// 說明書裡也查得到，含結局的中止碼。
	s.Key(ui.KeyRef)
	n := len(s.Menu.Items)
	for i := 0; i < n-1; i++ {
		s.Key(ui.KeyDown)
	}
	s.Key(ui.KeyConfirm)
	all := strings.Join(s.Menu.Items, "|")
	for _, want := range []string{"MEENU", "KEYS", "DRUIDS", "WAFE"} {
		if !strings.Contains(all, want) {
			t.Errorf("謎題那一頁找不到 %q", want)
		}
	}
}

// 世界地圖那一頁：按 W 進得去、標得出隊伍所在的區域、任意鍵離開。
//
// 網格是從玩家自己那份 `ATTRIB.DAT` 算出來的，所以這條同時證明
// `game.WorldGrid` 與 UI 之間接得上。
func TestWorldPage(t *testing.T) {
	s := load(t)
	if !s.Key(ui.KeyWorld) || s.Mode != ui.ModeWorld {
		t.Fatalf("按 W 沒有進世界地圖，現在是 %v", s.Mode)
	}
	if s.Draw() == nil {
		t.Fatal("世界地圖畫不出來")
	}
	// 起點在米德格特（地圖 0），四面自指，不在網格上。
	s.Game.World.MapIndex = 11
	info := s.Draw()
	if info == nil {
		t.Fatal("換到野外圖之後畫不出來")
	}
	if !s.Key(ui.KeyConfirm) || s.Mode != ui.ModeExplore {
		t.Errorf("離開世界地圖之後是 %v", s.Mode)
	}
}
