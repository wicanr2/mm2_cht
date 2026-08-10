# 統一譯名表

**專有名詞以手冊為準。** 手冊掃描已整理成
[`docs/manual/part-1.md`](../docs/manual/part-1.md) 到 `part-5.md`，
官方譯名一律優先於自行音譯的版本。手冊自己前後不一致時（例如
Middlegate 有「米德革特」與「米德格特」兩種寫法），下表註明取哪一個。

## 世界與地名

| 原文 | 譯名 | 說明 |
|---|---|---|
| CRON | 科隆 | 遊戲世界本體。手冊上冊 p.1 |
| Middlegate | 米德格特 | 起始城鎮。手冊上冊 p.32／p.33 作「米德革特」，p.37 與地圖集 p.1 作「米德格特」，取後者 |
| Atlantium | 亞特蘭汀 | |
| Tundara | 桑達拉 | |
| Vulcania | 佛卡尼亞 | |
| Sandsobar | 桑德索巴 | |
| Castle Pinehurst | 潘赫特城堡 | |
| Castle Hillstone | 海爾斯通城堡 | |
| Castle Woodhaven | 森林堡 | |
| Luxus Palace Royale | 皇家豪華宮殿 | |
| Murray's Resort Isle | 墨里休閒小島 | |
| Isle of the Ancients | 古代之島 | |
| Dead Zone | 死城 | |
| Square Lake | 方形湖 | |
| Lost Souls Woods | 靈魂森林 | |
| Corak's Wood | 科拉克森林 | 地圖標註用「科拉克」，內文人名用「柯拉克」，手冊自身不一致；地名沿用地圖 |
| Ambush Valley | 埋伏谷 | |
| Desert of Desolation | 荒涼沙漠 | |
| Nomadic Rift | 諾曼斷崖 | |
| Arcane Wilderness | 神秘之荒野 | |
| Plains of Peril | 危險之草原 | |
| Barbaric Hills | 野蠻山丘 | |
| Forbidden Forest | 森林禁地 | |
| Pearl Islands | 珍珠島 | |
| Native's Cove | 土著灣 | |
| Dawn's Mist | 迷霧沼澤 | |
| Corpus Bay | 屍首灣 | |
| Cronian Waters | 科隆尼亞湖 | |
| Emerald Coast | 翡翠海岸 | |
| Queen's Orchard | 女王的果園 | |
| Quagmire of Death | 亡命沼澤 | 地圖上作 Quagmire of Doom，手冊自身不一致 |
| Elemental Plane of Air / Fire / Water / Earth | 風／火／水／土元素領域 | |

## 人物

| 原文 | 譯名 |
|---|---|
| Corak the Mysterious | 柯拉克 |
| Sheltem | 席頓 |
| King Kalohn | 卡隆國王 |
| Princess Lamanda | 拉曼達公主 |
| Gwyndon | 昆登 |
| Acwalandear | 亞郭蘭大 |
| Pyrannaste | 皮倫奈斯特 |
| Shalwend | 雪文 |
| Gralkor the Cruel | 葛拉格 |
| Lord Pinehurst | 潘赫特城主 |

## 職業

| 原文 | 譯名 |
|---|---|
| Knight | 武士 |
| Paladin | 遊俠 |
| Archer | 弓箭手 |
| Cleric | 牧師 |
| Sorcerer | 巫師 |
| Robber | 賊 |
| Ninja | 忍者 |
| Barbarian | 野蠻人 |

## 屬性

| 原文 | 譯名 |
|---|---|
| Might | 力量 |
| Intellect | 智慧 |
| Personality | 人格 |
| Endurance | 耐力 |
| Speed | 速度 |
| Accuracy | 準確度 |
| Luck | 運氣 |
| Hit Point | 生命點數 |
| Experience Level | 經驗等級 |
| AC | 防護等級 |
| Thievery | 盜行 |

## 種族與陣營

| 原文 | 譯名 |
|---|---|
| Human | 人類 |
| Elf | 精靈 |
| Dwarf | 矮人 |
| Gnome | 侏儒 |
| H-Orc | 半獸人 |
| Good / Neutral / Evil | 善良／中立／邪惡 |

## 設施

| 原文 | 譯名 |
|---|---|
| Inn | 旅店 |
| Blacksmith | 鐵匠鋪 |
| Tavern | 酒館 |
| Temple | 神殿 |
| Training | 訓練所 |
| Mage Guild | 法師公會 |

## 刻意不翻的條目

| 類別 | 處理 | 理由 |
|---|---|---|
| 交織密碼的密文行（`Green/Yellow/Red Message` 之後那幾行亂碼） | 原樣保留 | 謎題的解法是把字母照特定順序重排。翻成中文之後那個機制就不存在了，玩家永遠解不開 |
| 座標與傳送表（`C2/3 C3 (1,9)`、`D1 (14,1) 4`） | 原樣保留 | 純編號，翻譯只會製造對不上的風險 |
| 字串切分產生的單字元碎片（`A`、`z`、`"`） | 原樣保留 | `STR.DAT` 沒有逐條索引（是區塊加順序游標），每一條是一行顯示不是一則訊息，切出這些碎片是切分粒度的產物，不是真正的訊息 |
| `(y/n)` 按鍵提示 | 保留英文按鍵，前後文譯成中文 | 原版按的就是 Y/N 兩個鍵，改成中文玩家會不知道要按什麼 |
| 製作群的人名與公司名（`Jon Van Caneghem`、`New World Computing, Inc.`） | 原樣保留 | 職稱與說明文字譯成中文，名字本身不譯 |
| 控制室謎題的答案 `Preamble` | 原樣保留 | 玩家要輸入的就是這個英文單字，譯了就答不出來 |
| 資料檔名（`monsters.16`、`eventsi.dat`） | 不進待譯清單 | 翻了遊戲開不了檔。抽取階段就過濾掉（`internal/assets/exetext`）|

## 換行標記

原文的 `@` 是換行。中文行寬與英文不同，譯文的 `@` 依中文長度重新安排，
不硬跟原文的斷行位置。

## 留白行

一段訊息在原版佔幾行，中文常常用不到那麼多行。用不到的那一行譯文寫一個
半形空白，不寫空字串 —— 空字串在管線裡等於「還沒翻」，會讓完成度永遠差幾條。

## MM2.EXE 尾部的字串（`exe.XXXX`）

城鎮、職業、種族、陣營、次要技能、身體狀況、戰鬥播報、陷阱訊息、
選單提示都在 `MM2.EXE` 尾部的 DGROUP 初值段，共 787 條。
key 用 DGROUP 偏移（`exe.0E2C`），抽取規則調整了也不會整批位移。

幾組要跟手冊對齊的譯名：

| 類別 | 依據 |
|---|---|
| 七項屬性（力量／智慧／人格／耐力／速度／準確度／運氣） | 手冊上冊 p.21。手冊自己對 Accuracy 有「準確度／精確度／準確性」三種寫法，統一取**準確度** |
| 十五項次要技能（武器專家／運動家／製圖家／宗教家／外交家／賭徒／鬥士／勇士／語言家／商人／登山家／領航員／探險家／扒手／戰士） | 手冊下冊 p.26 |
| 十一種身體狀況（正常／詛咒／沈默／疾病／中毒／沈睡／痲痺／無意識／死亡／石化／根除） | 手冊的休息限制清單 |

標題畫面原版是 `MIGHT` / `and` / `MAGIC` 三行，中文取「魔」「法」「門」
三個字直排，行數與版面都對得上。
