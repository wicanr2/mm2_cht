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

本機已有 Mega Drive ROM 與播放鏈的 IDA 證據，但目前沒有固定版本的 BlastEm／VGM
擷取 Docker image，也沒有 16 首曲目的可重播觸發腳本。因此「播放器與本機包契約」
已完成，「從 ROM 自動產生完整包」仍未完成，不能用網路曲庫下載冒充可重現輸出。

後續最小工作是建立固定版本 BlastEm image，以 ROM 唯讀掛載，逐首記錄 VGM，再在
同一工具鏈轉為 WAV。每首須保存 ROM hash、模擬器版本、VGM/WAV hash、觸發步驟與
取樣率。ROM、VGM、WAV 與完整版只可留在 `workplace/` 或 `.local-full/`，不得
commit、push 或放進公開包。
