#!/usr/bin/env python3
"""以真實 HD 成品產生 README 展示圖與完整圖集；不生成或重繪素材。"""
import argparse
import hashlib
import json
import math
from functools import lru_cache
from pathlib import Path

from PIL import Image, ImageChops, ImageDraw, ImageFont, ImageOps

BG = '#101b29'
PANEL = '#1b2b3d'
GOLD = '#dfc188'
WHITE = '#dfe7f0'

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--hd-dir', type=Path, default=Path('workplace/hd-amiga-r1'))
    ap.add_argument('--out', type=Path, default=Path('docs/gallery'))
    ap.add_argument('--font', type=Path, required=True)
    ap.add_argument('--screenshot', type=Path, required=True)
    args = ap.parse_args()
    mf = json.loads((args.hd_dir/'hd.json').read_text())
    entries = mf['entries']
    assert mf['expected'] == mf['generated'] == len(entries) == 930
    unique = len({e['art'] for e in entries})
    args.out.mkdir(parents=True, exist_ok=True)
    @lru_cache(maxsize=16)
    def font(size):
        return ImageFont.truetype(str(args.font), size, index=3)
    images = {}
    for e in entries:
        path = args.hd_dir/e['art']
        if e['art'] not in images:
            assert sha(path) == e['art_sha256'], e['id']
            images[e['art']] = Image.open(path).convert('RGBA')

    def paste(canvas, image, box):
        image = ImageOps.contain(image, (box[2]-box[0], box[3]-box[1]), Image.Resampling.LANCZOS)
        canvas.paste(image, (box[0]+(box[2]-box[0]-image.width)//2, box[1]+(box[3]-box[1]-image.height)//2), image if image.mode=='RGBA' else None)

    def tile(canvas, entry, box, label_size=17):
        d = ImageDraw.Draw(canvas)
        d.rounded_rectangle(box, radius=6, fill=PANEL)
        paste(canvas, images[entry['art']], (box[0]+7,box[1]+7,box[2]-7,box[3]-29))
        rgb = images[entry['art']].convert('RGB')
        red, green, blue = rgb.split()
        if ImageChops.lighter(ImageChops.lighter(red,green),blue).getextrema()[1] <= 12:
            d.text((box[0]+15,(box[1]+box[3])//2-22),'空白影格',font=font(18),fill='#8e9eaf')
        d.text((box[0]+8,box[3]-24),entry['id'],font=font(label_size),fill=WHITE)

    title = next(e for e in entries if e['source']=='intro.32')
    monsters = {}
    for e in entries:
        if e['kind']=='monster':monsters.setdefault(e['source'].split('_f')[0],e)
    scene_sources = ['town.32','townf.32','townt.32','cave.32','cavef.32','cavet.32','castle.32','castlef.32','castlet.32','desert.32','ocean.32','swamp.32','tundra.32','outdoor1.32','outdoor2.32','outdoor3.32','sky.32','outb.32','outf.32','townb.32']
    samples = [next(e for e in entries if e['source']==s and e['index']==0) for s in scene_sources]
    cover = Image.new('RGB',(2400,2040),BG)
    d = ImageDraw.Draw(cover)
    d.text((48,22),'Amiga HD 素材總覽',font=font(48),fill=GOLD)
    d.text((48,88),f'場景與介面 400 張　｜　怪物動畫 530 幀　｜　獨立畫稿 {unique} 幅',font=font(27),fill=WHITE)
    paste(cover,images[title['art']],(48,140,1180,650))
    paste(cover,Image.open(args.screenshot).convert('RGB'),(1220,140,2352,650))
    d.text((48,662),'場景、地形與火炬',font=font(29),fill=GOLD)
    for i,e in enumerate(samples):
        x,y=48+(i%10)*232,710+(i//10)*148
        tile(cover,e,(x,y,x+220,y+136))
    d.text((48,1015),f'怪物造型　{len(monsters)} 組，以下各取一幀',font=font(29),fill=GOLD)
    for i,e in enumerate(monsters.values()):
        x,y=48+(i%12)*194,1065+(i//12)*150
        tile(cover,e,(x,y,x+182,y+138),15)
    d.text((48,1985),'完整 930 筆素材與動畫影格另附圖集，可放大檢視。',font=font(23),fill=WHITE)
    cover.save(args.out/'amiga-hd-overview.webp',quality=88,method=4)

    groups = {'片頭與介面':[], '城鎮':[], '地城與城堡':[], '野外與自然':[], '怪物與動畫':[]}
    interface = {'intro.32','introclips.32','book.32','nwcp.32','throw.32','endgame.32'}
    for e in entries:
        source=e['source']
        group='怪物與動畫' if e['kind']=='monster' else '片頭與介面' if source in interface else '城鎮' if source.startswith('town') else '地城與城堡' if source.startswith(('cave','castle')) else '野外與自然'
        groups[group].append(e)
    columns,cell,margin = 20,152,32
    height=600+sum(60+math.ceil(len(items)/columns)*140 for items in groups.values())+100
    atlas=Image.new('RGB',(columns*cell+margin*2,height),BG)
    d=ImageDraw.Draw(atlas)
    d.text((margin,20),'Amiga HD 完整素材圖集',font=font(43),fill=GOLD)
    d.text((margin,83),f'共 {len(entries)} 筆對應、{unique} 幅獨立畫稿，另含兩種邊框；包含備存的片頭及結局配件。',font=font(24),fill=WHITE)
    for index,(key,hashkey,label) in enumerate([('path','sha256','一般介面邊框'),('page','page_sha256','全頁介面邊框')]):
        path=args.hd_dir/mf['frame'][key]
        assert sha(path)==mf['frame'][hashkey]
        left=margin+index*1520
        d.text((left,143),label,font=font(28),fill=GOLD)
        paste(atlas,Image.open(path).convert('RGBA'),(left,185,left+1480,560))
    y=600;seen=[]
    for name,items in groups.items():
        d.text((margin,y),f'{name}　{len(items)} 筆',font=font(30),fill=GOLD)
        y+=60
        for i,e in enumerate(items):
            x=margin+(i%columns)*cell;top=y+(i//columns)*140
            tile(atlas,e,(x,top,x+144,top+132),13)
            seen.append(e['id'])
        y+=math.ceil(len(items)/columns)*140
    assert len(seen)==930 and len(set(seen))==930
    atlas.save(args.out/'amiga-hd-complete.webp',quality=86,method=4)
    receipt={'source_manifest_sha256':sha(args.hd_dir/'hd.json'),'screenshot_sha256':sha(args.screenshot),'font_sha256':sha(args.font),'entries':len(seen),'unique_art':unique,'monster_groups':len(monsters),'outputs':{p.name:{'sha256':sha(p),'bytes':p.stat().st_size} for p in [args.out/'amiga-hd-overview.webp',args.out/'amiga-hd-complete.webp']}}
    Path('workplace/toolbar-font/overview-receipt.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(receipt,ensure_ascii=False))

if __name__=='__main__':main()
