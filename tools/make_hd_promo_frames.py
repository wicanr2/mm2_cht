#!/usr/bin/env python3
"""建立推廣片的原生版面與字幕，不改動遊戲截圖。容器掛載 Noto 字型。"""
from pathlib import Path
import json
import sys
from PIL import Image, ImageDraw, ImageFont

out=Path(sys.argv[1]);out.mkdir(parents=True,exist_ok=True)
version=Path('/src/VERSION').read_text().strip()
fontfile='/fonts/NotoSansCJK-Regular.ttc'
font=lambda size:ImageFont.truetype(fontfile,size,index=3)
scenes=[
 ('片頭重繪動畫',['熟悉的獨角獸','再次迎接冒險。'],10),
 ('走入 Cron 世界',['Amiga 素材','手繪重製','石雕與古銀邊框'],10),
 ('隨手調整畫面',['繁體與簡體','原圖與 HD','文字與工具列字級'],10),
 ('城鎮裡的日常',['交易裝備','整備隊伍','探訪城鎮設施'],10),
 ('探索與尋路',['區域地圖','彩色世界地圖','記住走過的路'],8),
 ('中文輸入',['繁簡中文姓名','輸入法組字','存檔保留原字'],8),
 ('回合制戰鬥',['隊伍分工','選擇攻擊目標','手繪怪物動畫'],12),
 ('準備再次出發',['Linux','Windows','macOS'],4),
]
for i,(title,lines,duration) in enumerate(scenes):
 im=Image.new('RGBA',(1920,1080),(10,18,31,255));d=ImageDraw.Draw(im)
 d.rectangle((0,0,1920,5),fill='#b69552')
 d.text((44,26),'魔法門 II',font=font(44),fill='#e2c68b')
 d.text((1460,42),version,font=font(24),fill='#a4b2c7')
 d.rounded_rectangle((1460,110,1876,1028),radius=12,fill='#111f30',outline='#34404f',width=2)
 d.text((1490,150),f'{i+1:02d} / 08',font=font(28),fill='#c6a563')
 d.text((1490,220),title,font=font(32),fill='#f4dfad')
 d.rectangle((1490,286,1838,288),fill='#9e7840')
 for j,line in enumerate(lines):d.text((1490,338+j*74),line,font=font(30),fill='#e3e8ef')
 if i==7:d.text((1490,670),'三平台完整版',font=font(30),fill='#e2c68b')
 d.text((44,1040),'手繪 HD｜繁簡中文｜三平台',font=font(23),fill='#a7b5c8')
 d.rectangle((41,107,1420,1029),outline='#9c814d',width=2)
 d.rectangle((44,110,1417,1025),fill=(0,0,0,0))
 im.save(out/f'frame-{i:02}.png')
(out/'storyboard.json').write_text(json.dumps([{'scene':i,'title':title,'lines':lines,'duration':dur} for i,(title,lines,dur) in enumerate(scenes)],ensure_ascii=False,indent=2)+'\n')
