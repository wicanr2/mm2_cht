# 《魔法門 II》繁中 remake 推廣片

推廣片採用 `retro-cht/game-promo-video-ffmpeg` 的三段式流程：先由目前引擎拍攝
實際 UI，再在容器中產生章節卡片，最後以固定畫面剪輯與音訊合成。分鏡刻意使用
紅門／EGA 色票（`#090606`、`#970404`、`#5B0403`、`#2F46BF`、`#505AF3`、
`#A75505`、`#AFDDEF`），並以 DOS／Amiga／MSX／現代 Theme 的並置強化遊戲介紹。

## 72 秒分鏡

| 時間 | 畫面 | 介紹重點 |
|---|---|---|
| 0–11 秒 | 紅門片頭、繁中標題、證據流程 | 從原版參照（oracle）到可玩的 remake |
| 11–31 秒 | 對話、第一人稱與 DOS／Amiga／MSX／Theme 畫面 | 多平台素材組與可替換主題 |
| 31–55 秒 | 建角、物品、商店、施法、戰鬥 | 正常 UI 的玩家操作鏈 |
| 55–69 秒 | 怪物研究卡與正常戰鬥 UI 動畫、防護、地圖、說明書 | 已解出的動畫資料與 remake 接線 |
| 69–72 秒 | 世界地圖與紅門收束 | 自備合法原版資料與公開邊界 |

畫面只放目前引擎的實際截圖或明確標示的研究卡。怪物卡的 `433 張影格` 與
`181 段序列` 是 `MONSTERS.16` 的資料稽核結果；緊接著的兩個時間點則由正常
`Session.Draw` 戰鬥路徑產生，中間以 `Session.Tick` 依 hold 前進。remake 目前選
每張圖第一個合法序列循環播放，屬強推論；原版哪一段對應待機、攻擊或受擊仍未知。

## 原版配樂與權利紀錄

- 音源輸入：玩家自備、未進版控的 `MM2.EXE`，SHA-256
  `631facb658a39e0d438c451f8a43c9f6e2aeb774fc3843c1a9bac1e14bf8c4d4`。
- 證據位置：DOS DGROUP base `0x8630`；曲目指標表 `DS:5214`，音高表 `DS:5144`，
  時值表 `DS:51F4`。十個曲目指標與三張表都是 `MM2.EXE` 的 DGROUP 位址空間，
  對應檔案偏移為 `DS + 0x8630`。
- 時值證據：`TIMER.DRV` SHA-256
  `63cdfe8605caa94ef4c0d399c1a0ffcec5d223ae2701958526d203a9e419b812`；IDA Pro 9.4
  的 `TIMER.DRV` 位址空間中，`0x181–0x18C` 將 PIT channel 0 divisor 設成
  `0x0400`，`0x25E` 讀取時值，`0x1D5` 於每次 IRQ 遞減。因此 `DS:51F4`
  是 PIT tick，不是毫秒；一 tick 以 `1024 / 1,193,182` 秒換算。這些語意皆為
  已證實，並保留原始位址以便回查。
- `cmd/mm2music` 驗證上述 EXE 雜湊後，讀取原始音高／時值資料，離線產生單聲道
  PC 喇叭（PC speaker）方波 WAV；推廣片使用索引 `3–8` 串成組曲（medley），並循環
  至片長。這是從原版資料重建的本機轉譯，不是 DOSBox 的原始錄音，也不是完整原版
  配樂集（soundtrack）。
- 這份 63 檔的 DOS 發行資料沒有 MT-32／MIDI／MPU-401 驅動或音樂資料；`MM2.EXE`
  與 `TIMER.DRV` 的已證實播放路徑寫 PIT channel 2 與 PC speaker port `0x42`／`0x61`，
  沒有 MPU-401 `0x330` I/O。這個結論只適用於本專案的 DOS oracle，不外推至 Amiga、
  MSX 或後來的非官方重編曲。
- WAV、影片與原版畫面一律輸出到被 Git 忽略的 `workplace/promo/`；不得提交或上傳。
  若要公開宣傳片，必須改用可再散布的原創 Theme／音源並重新驗收。
- Mega Drive 版的 16 首場景配樂已經擷取成本機音樂包，推廣片因此有兩個配樂變體：
  `mm2-remake-trailer.mp4` 是上面那段 DOS PC 喇叭組曲，
  `mm2-remake-trailer-megadrive.mp4` 用 Mega Drive 的 `intro` 與 `town` 兩首交疊成
  72 秒。畫面完全相同，只換音軌。**兩個都是原版衍生內容**，一樣只留在
  `workplace/promo/`。本機音樂包契約見 [`music.md`](music.md)。

## 重拍

```bash
bash tools/render_promo.sh --data-dir /path/to/MM2
```

腳本只在 Docker 內執行 Go、截圖、WAV 轉譯與 FFmpeg。輸出包括：

```text
workplace/promo/mm2-remake-trailer.mp4              DOS PC 喇叭組曲
workplace/promo/mm2-remake-trailer-megadrive.mp4    Mega Drive 配樂（有音樂包才有）
workplace/promo/music/mm2-original-pc-speaker.wav
workplace/promo/music/mm2-megadrive-medley.wav
workplace/promo/shots/*.png
```

音樂包預設找 `workplace/genesis/music`，`--md-music-dir` 可以換一份，
`--no-md-music` 只出 PC 喇叭那一版。

完成後需人工觀看與聆聽，確認中文沒有裁切、Theme 標示與實際畫面一致、字幕沒有把
研究證據說成產品完成，且方波音色沒有蓋過未來可能加入的遊戲音效。技術上可解碼不
等於完成觀賞驗收。
