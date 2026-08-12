# 公開釋出

`wicanr2/mm2_cht` 目前是 private，第一個可玩切片之後才轉公開。
公開的產出只有**引擎程式碼、翻譯文本與可公開的原創內容**，玩家自備合法原版。

## 歷史裡的五份衍生資料

`data/creation.json`、`data/experience.json`、`data/pictures.json`、
`data/terrain.json`、`data/traps.json` 曾經被 commit 進來，後來才 gitignore，
所以**還留在 git 歷史裡**。五份都是從 `MM2.EXE` 的 DGROUP
抽出來的查表，每一份自己帶 `"source"` 欄位指向來源位址：

| 檔 | 內容 | 來源 |
|---|---|---|
| `creation.json` | 建角的生命、法力、防護與神殿倍率表 | `ds:06E6`／`06F2`／`071E`／`074D`／`46A8` |
| `experience.json` | 升級所需經驗 | `ds:2E5C` ＋ `sub_CC8C` |
| `pictures.json` | 圖片編號表 | `ds:164C`／`1662`／`167C`／`1694`／`16AC` |
| `terrain.json` | 野外地形分類 | `ds:52B2` |
| `traps.json` | 陷阱傷害與訊息 | `ds:2946`／`ds:28F2` |

## 決定：私有研究與公開專欄並行

使用者於 2026-08-12 選擇「C＋A」：

- **私有研究（C）**：可在 `docs/research/soft-world/` 補齊《軟體世界》
  MM2 連載的逐字研究稿、頁碼索引、辨讀不確定處與地圖研究；掃描與原始輸入
  維持工作區外或 gitignore 路徑，不建立公開副本。
- **公開專欄（A）**：可發布自行撰寫的導讀、改寫後的攻略摘要、驗證紀錄、
  勘誤與來源頁碼；不得重印雜誌正文、表格、地圖、掃描或可替代原文的逐字 OCR。

公開版必須排除 `docs/research/soft-world/`。該目錄內的研究稿即使是純文字，
仍是未取得重發布授權的雜誌內容；可公開瀏覽的預覽頁不構成轉載許可。
`tools/check_release.sh --public` 會把它視為硬性失敗。專欄本身不主張、也不取代
任何雜誌內容的公開授權。

現有 `data/hints.json` 由上述私有研究稿整理而成，公開版也不得直接帶出；
少了它時遊戲會安全地略過提示功能。若未來要在公開版恢復提示，必須逐條改寫、
重新審核為原創摘要，並在 `docs/columns/soft-world/` 留下頁碼出處與證據等級。

## 決定：不改寫歷史，公開時另開乾淨 repo

使用者 2026-08-10 定案。私有工作 repo 保留完整開發歷史不動，
公開版另建一份乾淨的。

考慮過而沒有採用的是 `git filter-repo --invert-paths` 加 force push：
它會改寫 138／204 個 commit，而收益只是省下另開一份 repo 的工。
兩者的公開結果一樣乾淨，差別在私有歷史要不要被破壞。

## 公開時的步驟

1. 在乾淨目錄 `git init`，把工作區檔案複製過去（不含 `.git`、
   `workplace/`、原版壓縮檔、`data/*.json` 裡由原版產生的那些）。
2. 確認 `.gitignore` 與工作 repo 一致。
3. **在那份新 repo 裡跑 `tools/check_release.sh --public`** ——
   所有 deny-list 檢查全過才推。`--public` 會把「歷史裡有衍生資料」與
   私有雜誌逐字研究稿當成硬性失敗。
4. 推到公開 repo。

工作 repo 上直接跑 `tools/check_release.sh`（不帶旗標）只把第三項
當成現況報告，不擋 —— 它報告的是**已知且已決定接受的狀態**，
不是待辦事項。

## 什麼東西一定不能進公開 repo

- 原版執行檔、`.OVL`、`.DAT`、`.16`、`.CH`、`.DRV`
- 原版壓縮檔（`.zip`／`.rar`／`.dsk`）與說明書掃描
- 反組譯產物（`.i64`／`.asm`）與解包後的 `workplace/`
- `cmd/mm2data` 產生的任何 `data/*.json`（帶 `"source"` 指向原版的那些）
- `docs/research/soft-world/` 的逐字研究稿，以及雜誌掃描、OCR 原文、表格與地圖
- 仍以《軟體世界》研究稿為來源的 `data/hints.json`（未逐條重寫／審核前）
- IDA 的 license

手抄自手冊的資料（職業成長、法術說明）不在此列，那些入版控。
