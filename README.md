# Might and Magic II: Gates to Another World — 繁體中文 remake

把 New World Computing 1988 年的《Might and Magic II》完整逆向，在 Go / Ebiten 上重寫引擎，
再做繁體中文化。定位是文化資產保存。

## 現況

專案剛建立，尚未開始逆向。目前 repo 內只有工作規範（[`CLAUDE.md`](CLAUDE.md)）與 IDA 包裝腳本。

已驗過的起點事實（`MM2.EXE` 未打包、14 個 `.OVL` 是裸機器碼段、Phoenix overlay runtime、
檔名大小寫不一致）記在 `CLAUDE.md` §3，第一批要解的未知記在 §4。

## 做法

- 原版 DOS 執行檔是唯一的規則裁決者。手冊與攻略是佐證，可能描述其他平台。
- 反組譯 → 收攏成規格 → 才實作。只有標 `READY` 的規格可以動手。
- 每個斷言標推論等級（已證實／強推論／假設／未知），`已證實` 要附得出證據。
- 原版 320×200 EGA 骨架與素材保持不動，中文走獨立的高解析點陣疊加層。

## 授權與素材

不散布原版執行檔、資料檔、美術或音樂。公開產出只有引擎程式碼與翻譯文本，玩家自備合法原版。
原版資料一律 gitignore。

## 姊妹專案

[demon_winter_cht](https://github.com/wicanr2/demon_winter_cht) — SSI《Demon's Winter》(1988) 繁中 remake，
本專案的方法論、工具鏈與分層架構沿用自它。
