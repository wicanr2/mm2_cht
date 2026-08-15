# 酒館傳聞

MM2 的酒館有兩處會吐線索：**選項 D 花 1 金打聽**、**選項 E 免費打聽**，
兩邊各有一組表，五座城各四則。這份文件是那 40 則的中英對照。

機制與位址見 [`re/04-2brain-tavern.md`](re/04-2brain-tavern.md)，這裡只放內容。
英文取自 `STR.DAT`，中文取自 `translations/zh-Hant.json`；一則佔兩筆字串時
兩行併成一格。

## 今天講哪一則

兩個選項共用同一支挑選函式（`sub_1CA46`），依「今天」算索引：

	今天 = ds:03A2[ds:03CA × 2]        與 opcode 0x23 判日期讀的同一格
	今天 == 180        → 第 3 則
	今天 能被 30 整除  → 第 (今天 ÷ 30) 則
	否則               → 第 ((今天 & 1) ^ 1) 則（偶數日 1、奇數日 0）

所以**平常只輪得到前兩則**：第 2 則要第 60 天、第 3 則要第 90 天或第 180 天。
第 120／150 天算出來的索引是 4 與 5，**超出四則的範圍** —— 原版沒有防護，
讀到的是表後面的東西。選項 D 另外要擲 `rand(1, 屬性修正(耐力) + 5)` 擲出 1
才肯講，沒擲中那 1 金照樣付掉。

## 簡寫怎麼讀

| 寫法 | 意思 |
|---|---|
| `A1`–`E4` | 野外區域碼（飛行術的 5×4 表，見 [`formats/09-spells.md`](formats/09-spells.md)）|
| `C1`–`C4` | 城堡編號 |
| `S9,3`／`C2,3` | 法術：`S` 巫師系、`C` 牧師系，後面是等級與編號 |
| `Meal A/B/C` | 酒館的三道菜。**吃飯會在角色記錄 `+118` 留下旗標**，所以很多線索的前半段是要你先去吃那一餐 |
| `0,15` 這種數對 | 該地圖裡的 `(X, Y)` |

## 免費打聽（選項 E）

原版表 `ds:56D2`，`STR.DAT` 第 128–167 筆。

| 城 | 第 0 則 | 第 1 則 | 第 2 則 | 第 3 則 |
|---|---|---|---|---|
| 米德格特 | Children at 0,15<br>孩童在 0,15 | Goblets at 0,7<br>高腳杯在 0,7 | Meal A, then C1 2,10<br>A 餐，然後 C1 2,10 | Meal B, then C1 2,6<br>B 餐，然後 C1 2,6 |
| 亞特蘭汀 | Time travel at Pinehurst<br>時空旅行在 潘赫特 | Meal B, then A2 14,10<br>B 餐，然後 A2 14,10 | S7,1 B2 15,11<br>S7,1 B2 15,11 | Meal C, then C1 1,8<br>C 餐，然後 C1 1,8 |
| 桑達拉 | 'Murray's rejuvenates'<br>「墨里讓人返老還童」 | Hirelings at 15,10<br>雇傭兵在 15,10 | Cast C2,3 day 93 gain S9,3<br>第 93 天施 C2,3 可得 S9,3 | Meal C-an experience<br>C 餐是一場體驗 |
| 佛卡尼亞 | B2, day 140-170, 14,4<br>B2，第 140-170 天， 14,4 | C3,6 C2 11,1<br>C3,6 C2 11,1 | Lord Haart B1 5,5<br>哈特領主 B1 5,5 | Meal B is a riot<br>B 餐熱鬧非凡 |
| 桑德索巴 | Mandagaul D2 6,8<br>曼達高爾 D2 6,8 | The Gourmet A3 7,6<br>美食家 A3 7,6 | C9,2 C1 south<br>C9,2 C1 南方 | Meal C, 7,3-Sarakin<br>C 餐，7,3－薩拉金 |

## 花 1 金打聽（選項 D）

原版表 `ds:5676`，`STR.DAT` 第 168–207 筆。

| 城 | 第 0 則 | 第 1 則 | 第 2 則 | 第 3 則 |
|---|---|---|---|---|
| 米德格特 | See Nordon at 10,2<br>去 10,2 找諾登 | Donate at all temples<br>每座神殿都要捐獻 諾登有 S2,1 | Nordon has S2,1 | Meal C, then D1 2,7<br>C 餐，然後 D1 2,7 |
| 亞特蘭汀 | Hirelings at 0,14<br>雇傭兵在 0,14 | Transmutations 8,8 the corners<br>嬗變在 8,8 四個角落 | Castle Xabran holds all clues<br>薩布蘭城堡握有 所有線索 | Meal A, then C4 14,8<br>A 餐，然後 C4 14,8 |
| 桑達拉 | C2,3 C3 1,9<br>C2,3 C3 1,9 | Hoardall seeks items<br>霍達爾在找道具 | Keys add castle gold<br>鑰匙能增加城堡金幣 | Meal B, then C3 1,9<br>B 餐，然後 C3 1,9 |
| 佛卡尼亞 | Visit Dawn's, D4 3,7<br>去道恩那裡，D4 3,7 | Slayer seeks death<br>屠戮者尋求死亡 | S5,2 C1 1,8<br>S5,2 C1 1,8 | Meal C, then E1 2,3<br>C 餐，然後 E1 2,3 |
| 桑德索巴 | Tavern drinks give a bonus<br>酒館的酒能帶來 加成 | S3,6 7,4<br>S3,6 7,4 | Meal A, then 3,11<br>A 餐，然後 3,11 | Meal B, then E4 3,10<br>B 餐，然後 E4 3,10 |

## 這 40 則涵蓋了什麼

四十則裡有十四則是「先吃某一餐，再去某個座標」的兩段式線索 —— 那正好對上
餐點會在記錄 `+118` 留下旗標這件事。其餘是地點（雇傭兵、諾登、哈特領主）、
法術（`S7,1`、`C2,3`）、日期條件（`day 93`、`day 140-170`），
以及兩位領主各自要什麼（`Hoardall seeks items`、`Slayer seeks death`）。
