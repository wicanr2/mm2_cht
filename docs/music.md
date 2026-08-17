# 本機音樂主題

## 預設就是 Mega Drive

`cmd/mm2` 不給 `-music-pack` 也會自己找音樂包，依序試
`workplace/genesis/music/`、`workplace/music/`、執行檔旁邊的 `music/`，
找到就播、找不到就靜音（音檔是玩家自備的，缺了不是錯誤）。

**Mega Drive 排在最前面**：四個平台裡只有它的 16 首場景曲目是逐首從 ROM
擷取、逐一對到場景的。`-music-theme` 仍然可以指定，值與音樂包宣告的不符
就直接報錯 —— 那是防止「以為在聽 A 其實在聽 B」，不是偏好設定。

remake 的公開程式碼只定義「現在是城鎮、地城、戰鬥或設施」等音樂角色，
不附原版音檔。玩家可將自己合法持有的版本轉成 PCM WAV，放進未納入版控的
本機音樂包；任何音檔缺失、格式錯誤或路徑不安全時，整包會在啟動時拒絕。

## 已證實的平台差異

- Mega Drive：YM2612 ＋ SN76489，已證實完整 16 首場景／提示曲目；是主要基準。
- MSX：有 PSG 與 MSX-MUSIC／OPLL 序列器，至少六個正常路徑曲目編號；曲名對照
  尚未完整，因此部分包只能明確缺曲，不能冒稱完整。
- Amiga：有 `audio.device` 四聲道序列；曲目數與場景對照尚待動態 oracle。
- DOS：本專案資料是 PIT／PC Speaker 短曲，沒有 MT-32。

反組譯位址、輸入雜湊與外部交叉證據見
[`research/02-other-platforms.md`](research/02-other-platforms.md)。

## 十六個角色怎麼被觸發

十一個是**背景樂**（`MusicCue()` 依畫面回傳，換了就換曲）：

| 角色 | 什麼時候 |
|---|---|
| `town` | 地圖 0–4（五座城鎮）|
| `dungeon` | 室內、場景碼 1（`cave*.16` 那組牆）|
| `castle` | 室內、場景碼 2 或 5（`castle*.16`）＝ 地圖 45–59 |
| `outside` | 其餘室外 |
| `battle` | 戰鬥中 |
| `inn`／`blacksmith`／`tavern`／`temple`／`training` | 進對應的設施選單 |

判準是**原版的場景碼**不是猜的：`2PLAY _2play_e10` 是 7 個 case 的 switch，
case 0 推 `town*.16`、case 1 推 `cave*.16`、**cases 2 與 5 推 `castle*.16`**、
cases 3/4/6 推 `out*.16`。檔名是把 DGROUP 初值段的指標解出來讀的 ——
反組譯裡那些 `dw` 是程式碼被當成資料的誤讀，照著讀會得到 `'t item'`
這種東西。

五個是**一次性音效**（stinger），播完回到原本的背景樂：

| 角色 | 什麼時候 |
|---|---|
| `enemy_killed` | 戰鬥中消滅一隻敵人 |
| `member_killed` | 戰鬥中隊員陣亡 |
| `victory` | 打贏 |
| `defeat` | 全隊倒下 |
| `treasure` | 按 `S` 撿到戰利品 |

**一幀只播一個**：同一回合又打贏又死了人時，依
`defeat > victory > treasure > member_killed > enemy_killed` 挑。
疊著播會變成噪音。

`intro` 接在片頭畫面（開頭那隻獨角獸在吃草那張）—— 按任意鍵離開片頭，
背景樂就換成當下場景該有的曲子。

> ⚠ 戰鬥結算那幾首（`victory`／`defeat`／`enemy_killed`／`member_killed`／
> `treasure`）的 Mega Drive 曲目對照**是強推論**：當初是在模擬器裡量的，
> 但沒有逐一走到那些時機驗證。先前它們播不到所以不影響交付，現在玩家
> 聽得到了，值得補一次動態驗證。

## 音樂包格式

音檔限 1 或 2 聲道、8 或 16-bit PCM WAV；播放時統一重採樣為 48 kHz、雙聲道。
`manifest.json` 範例：

```json
{
  "theme": "megadrive",
  "tracks": {
    "intro": "wav/intro.wav",
    "town": "wav/town.wav",
    "battle": "wav/battle.wav",
    "enemy_killed": "wav/enemy-killed.wav",
    "victory": "wav/victory.wav",
    "member_killed": "wav/member-killed.wav",
    "defeat": "wav/defeat.wav",
    "training": "wav/training.wav",
    "temple": "wav/temple.wav",
    "blacksmith": "wav/blacksmith.wav",
    "inn": "wav/inn.wav",
    "tavern": "wav/tavern.wav",
    "dungeon": "wav/dungeon.wav",
    "outside": "wav/outside.wav",
    "treasure": "wav/treasure.wav",
    "castle": "wav/castle.wav"
  }
}
```

Mega Drive 包必須包含上述 16 個角色。MSX、Amiga 與 DOS 可誠實標成部分包；正常
UI 進入沒有映射的場景時會靜音，不會沿用上一首曲子冒充。路徑必須相對於 manifest，
不得使用絕對路徑、`..`、反斜線、未知或重複角色。

```bash
go run ./cmd/mm2 -data <MM2> -music-pack <音樂包>/manifest.json
```

若要鎖定主題，可另加 `-music-theme megadrive`；空值採 manifest 自己宣告的主題。
`-music-theme off` 強制停用音樂。

## 擷取現況與權利邊界

**擷取工具鏈已完成並驗證**（2026-08-13）。固定版本的 Docker image
`mm2-blastem:0.6.3-pre-732f5689d438`（`docker/blastem/`）以 ROM 唯讀掛載，
一次執行就從 ROM 走到音樂包可以直接吃的 WAV：

```bash
tools/blastem_run.sh "wait:8;rec:intro;wait:25;stop"
```

四個環節都有獨立的量測面驗過，不是「跑得動就算」：

| 環節 | 版本／工具 | 驗收 |
|---|---|---|
| 模擬 | BlastEm `0.6.3-pre-732f5689d438` | 截圖看得到遊戲畫面 |
| 記錄 | 同上的 `ui.vgm_log` 熱鍵 | magic `Vgm ` 1.50、YM2612 7,670,453 Hz、SN76489 3,579,545 Hz、總取樣數換算秒數等於 timeline 給的秒數 |
| 轉檔 | libvgm `61fc6725644886abc3168e240e4e51588d74bdf7` 的 `vgm2wav` | 48 kHz／16-bit／立體聲 |
| 正規化 | `docker/blastem/vgm2pcmwav.py` | `vgm2wav` 寫的是 WAVE_FORMAT_EXTENSIBLE，Ebiten 會以 `wav: format must be linear PCM` 拒收；只重寫檔頭，取樣點不動 |
| 下游 | remake 自己的 `wav.DecodeWithSampleRate` | 實際解碼成功，不用別的播放器代替 |

**逐首觸發已完成**（`tools/md_music_dump.sh`）。18 首曲目位址是本專案自己解出來的，
擷取時用 GDB remote stub（`blastem ROM -D`）在 vblank 的音樂管理下中斷點，
每幀把 RAM `$FFCB62`（要播哪一首）寫成目標曲目，壓 40 幀並**驗證 `$FFCB5E`
（正在播）確實切過去**才開始錄。

**ROM 一個位元組都不改** —— 這片有開機時的完整性檢查，改動任何一個位元組
（尾端 padding 除外）就開不了機。細節見
[`research/md-music-driver.md`](research/md-music-driver.md)。

### 擷取結果（2026-08-13）

18 首全部擷取成功，零失敗。每首 40.2–40.4 秒、48 kHz／16-bit／立體聲，
RMS 12,342–13,589。**18 首的晶片寫入特徵兩兩不同**（YM2612 兩個埠與 SN76489
的寫入筆數組合，18/18 相異），所以確認是 18 首不同的曲子，不是同一首錄了 18 次 ——
單看檔案雜湊不同不足以排除「同一首但起點差幾個取樣」。

逐首的 VGM／WAV 雜湊與中介資料在 `workplace/genesis/music/manifest.txt`（不入版控）。

一個已知的量測結果：每首的峰值都是滿刻度（−32768）。chiptune 常態，
但若之後聽出削波，轉檔時要降增益重跑，不要在 WAV 上事後正規化。

### 曲目與場景的對照：已由反組譯解出

**16 個角色全部已定，來源是反組譯不是人耳。** 完整推導、證據與推論等級見
[`research/md-music-scenes.md`](research/md-music-scenes.md)：

| 角色 | 曲目 | 角色 | 曲目 |
|---|---|---|---|
| `town` | `0x0B48D4` | `victory` | `0x0BE238` |
| `dungeon` | `0x0B1370` | `enemy_killed` | `0x0B9718` |
| `outside` | `0x0B2290` | `member_killed` | `0x0B61DC` |
| `castle` | `0x0AF59C` | `defeat` | `0x0B9888` |
| `battle` | `0x0B8224` | `treasure` | `0x0B885C` |
| `inn` | `0x0BA608` | `training` | `0x0BD078` |
| `tavern` | `0x0BBF68` | `temple` | `0x0BC990` |
| `blacksmith` | `0x0B9A04` | `intro` | `0x0B8AE0` |

九個標**已證實**。四個區域主題（`town`／`dungeon`／`outside`／`castle`）是
兩份獨立資料互相印證：選曲依 `sub_FB86` 的地圖編號區間，而那些區間與 DOS 版
既有的地圖語意完全吻合。五個設施（`inn`／`blacksmith`／`tavern`／`temple`／
`training`）是**在模擬器裡走進去量的** —— 選曲函式下中斷點，記下 `d0` 與
回傳位址，配一張店內截圖。其餘標**強推論**，依據是呼叫端附近的字串錨點整批一致。

音樂包由 `tools/md_music_manifest.py` 產生，用 remake 自己的
`music.LoadManifestFile` 驗證過（16 個角色，theme=megadrive）。

### [HARD] 場景對照一律用反組譯推，不用人耳

「聽起來像城鎮」不是證據：換一個聽的人就換一個答案，沒辦法重跑，
資料更新後也沒辦法自動驗證。反組譯得到的是「程式在什麼情況下播這一首」，
那是遊戲自己的定義。整條推導寫成 `tools/md_music_scenes.py`，可以重跑並推翻。

推不出來的就寫「未定」並說明卡在哪，不要用聽的補上去再標成結論 ——
那會讓一個猜測混進一整批已證實的資料裡，而讀的人分不出來。

`tools/md_music_preview.py` 仍然留著，但用途改成**人耳抽查**（確認擷取沒有錯位、
沒有削波），不是決定角色歸屬。


在 16 首齊備之前**不得宣稱已有 Mega Drive 完整包**，也不能用網路曲庫下載冒充
可重現輸出。每首完成時要一併保存 ROM hash、模擬器版本、libvgm commit、
VGM/WAV hash、觸發步驟與取樣率。ROM、VGM、WAV 與完整版只可留在 `workplace/`
或 `.local-full/`，不得 commit、push 或放進公開包。

通用方法論（其他遊戲也適用的找法與坑）在
`~/.claude/knowledge-base/retro/megadrive-vgm-music-extraction.md`。
