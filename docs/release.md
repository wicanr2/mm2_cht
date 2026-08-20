# 公開釋出

**`wicanr2/mm2_cht` 是 public**（`gh api repos/wicanr2/mm2_cht --jq .visibility` 查得到）。
釋出包的內容只有**引擎程式碼、翻譯文本與原創圖示**，玩家自備合法原版；
repo 與攻略站的公開範圍另見下面兩節。

三平台打包骨架、Docker 命令與各平台真機閘門見
[`docs/packaging.md`](packaging.md)。

## 決定：repo 公開，攻略站發 GitHub Pages（2026-08-20）

使用者裁決，並且知道以下三件事：

- **GitHub Pages 的網址沒有存取控制**，跟 repo 是不是 private 無關。
  站上的頁面掛了 `noindex,nofollow`，那只擋搜尋引擎收錄，不擋任何拿到網址的人。
- 站上有《軟體世界》連載與珍017 說明書整理過的內容。兩者都**沒有取得重製授權**，
  這個 repo 只做整理、對照與考證，不重刊掃描或可替代原文的逐字全文。
- 站上那 25 張格位圖是由 `MAP.DAT` ＋ `ATTRIB.DAT` 逐格算出來的，
  **等於把原版 25 張地圖的完整牆面、門與事件格資料換一個格式公開**。
  它不含原版的一個位元組，但帶得走原版的資訊。

**因此本檔先前那條「公開版必須排除 `docs/research/soft-world/` 與 `data/hints.json`」
已經不成立** —— 兩者都在公開的 repo 裡，`data/hints.json` 整理出來的 280 條提示
也在站上。`tools/check_release.sh --public` 仍然會把它們判成硬性失敗，
那個模式現在只適用於「另建一份最小公開 repo」的場景，不是這一份工作 repo。

要**再擴大**公開範圍（例如把原版素材的解包結果、逐字掃描、`data/*.json` 的
原版衍生表放上去）時**先問**，不要拿這一節推廣解釋。

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

## 先前的決定：私有研究與公開專欄並行（已被上一節取代）

使用者於 2026-08-12 選擇「C＋A」：

- **私有研究（C）**：可在 `docs/research/soft-world/` 補齊《軟體世界》
  MM2 連載的逐字研究稿、頁碼索引、辨讀不確定處與地圖研究；掃描與原始輸入
  維持工作區外或 gitignore 路徑，不建立公開副本。
- **公開專欄（A）**：可發布自行撰寫的導讀、改寫後的攻略摘要、驗證紀錄、
  勘誤與來源頁碼；不得重印雜誌正文、表格、地圖、掃描或可替代原文的逐字 OCR。

這一條當初的前提是「工作 repo 是私有的」。**那個前提在 2026-08-20 已經不成立**：
repo 是 public，研究稿與 `data/hints.json` 都公開可讀，整理過的內容也在攻略站上。
`tools/check_release.sh --public` 仍會把它視為硬性失敗 —— 那個旗標現在只給
「另建一份最小公開 repo」用。**這個專案不主張、也不取代任何雜誌內容的公開授權。**

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
- 雜誌掃描、OCR 原文、原刊表格與原刊地圖
- IDA 的 license

`docs/research/soft-world/` 的逐字研究稿與 `data/hints.json` 原本也在這份清單上，
2026-08-20 起不在（見本檔第二節）。

手抄自手冊的資料（職業成長、法術說明）不在此列，那些入版控。
