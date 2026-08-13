# 本機音樂主題

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

仍未定的是**曲目與場景的對照**：`0x0B620` 那支跳表 switch 的 21 個 case 都解出來了，
但「哪個 case 代表哪個場景」要追它的呼叫端才知道。在那之前，音樂包的 16 個角色
還不能自動對應到這 18 首。

在 16 首齊備之前**不得宣稱已有 Mega Drive 完整包**，也不能用網路曲庫下載冒充
可重現輸出。每首完成時要一併保存 ROM hash、模擬器版本、libvgm commit、
VGM/WAV hash、觸發步驟與取樣率。ROM、VGM、WAV 與完整版只可留在 `workplace/`
或 `.local-full/`，不得 commit、push 或放進公開包。

通用方法論（其他遊戲也適用的找法與坑）在
`~/.claude/knowledge-base/retro/megadrive-vgm-music-extraction.md`。
