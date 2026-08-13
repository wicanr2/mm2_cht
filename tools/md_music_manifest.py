#!/usr/bin/env python3
"""照反組譯的結果產生 Mega Drive 音樂包的 manifest.json。

角色 → 曲目的對照**全部來自反組譯**（推導與證據見
docs/research/md-music-scenes.md），不是聽出來的。這支只做組裝：
檢查每個角色的 WAV 存在、寫出 manifest。

    md_music_manifest.py <音樂目錄>

音樂目錄裡要有 md_music_dump.sh 產出的 md-<曲目位址>.wav。
"""

from __future__ import annotations

import json
import os
import sys

# 角色 → 曲目位址。每一列的證據等級與依據見 docs/research/md-music-scenes.md。
ROLES = {
    # 已證實：case 19 依 sub_FB86(地圖編號) 的區間查表分支，
    # 而那些區間與本專案既有的 DOS 版地圖語意完全吻合。
    "town": "0B48D4",  # 地圖 0–4
    "dungeon": "0B1370",  # 地圖 17–32
    "outside": "0B2290",  # 地圖 5–16、33–44
    "castle": "0AF59C",  # 地圖 45–59
    # 已證實：在 Middlegate 逐一走進每個設施，中斷點記下 sub_B620 的 d0 與呼叫端。
    "inn": "0BA608",  # case 11：呼叫端 0x18BCA（走進 (7,3) 旅店入口）
    "blacksmith": "0B9A04",  # case 13：呼叫端 0x189B8（S. J. Blacksmith）
    "tavern": "0BBF68",  # case 14：呼叫端 0x1AA48（Slaughtered Lamb）
    "temple": "0BC990",  # case 15：呼叫端 0x1B94A 神殿、0x19D26 法師公會
    "training": "0BD078",  # case 16：呼叫端 0x1966C（Turkov's Training）
    # 強推論：呼叫端所屬函式 ＋ 附近字串錨點整批一致。
    "intro": "0B8AE0",  # case 10：主迴圈頂端的 sub_A4EE（標題／主選單）
    "battle": "0B8224",  # case 2：surprised you! / Power Shield / runs away!
    "victory": "0BE238",  # case 3：members receive（分經驗）
    "enemy_killed": "0B61DC",  # case 6：goes down! / is killed!
    "member_killed": "0B9718",  # case 8：dies horribly! / keels over!
    "defeat": "0B9888",  # case 7：Unfortunately, you were not successful
    "treasure": "0B885C",  # case 5：Each share is worth
}


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("用法: md_music_manifest.py <音樂目錄>")
    d = sys.argv[1]

    tracks, missing = {}, []
    for role, song in ROLES.items():
        name = f"md-{song}.wav"
        if os.path.exists(os.path.join(d, name)):
            tracks[role] = name
        else:
            missing.append((role, name))

    if missing:
        for role, name in missing:
            print(f"缺 {role}：{name}", file=sys.stderr)
        raise SystemExit(f"{len(missing)} 個角色的音檔不存在，先跑 tools/md_music_dump.sh")

    out = os.path.join(d, "manifest.json")
    with open(out, "w", encoding="utf-8") as fh:
        json.dump({"theme": "megadrive", "tracks": tracks}, fh, ensure_ascii=False, indent=2)
        fh.write("\n")

    used = sorted(set(ROLES.values()))
    print(f"{out}：{len(tracks)} 個角色，用到 {len(used)} 首曲子")


if __name__ == "__main__":
    main()
