#!/usr/bin/env python3
"""Docker-only asset packing. Artistic redraws are supplied by imagegen.

init exports read-only source bytes and contact sheets. ingest unpacks an
imagegen sheet, preserving the source mapping. It never invents game geometry.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import copy

from PIL import Image, ImageChops, ImageDraw, ImageFont
import amiga32

ROOT = Path("workplace/hd-amiga-r1")
CELL, COLS, ROWS = 512, 4, 4


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2)+"\n")


def decode(entry, palette):
    w, h, _, planes = entry
    stride = (w+15)//16*2
    pixels = []
    for y in range(h):
        for x in range(w):
            index = sum(((planes[(p*h+y)*stride+x//8] >> (7-x%8)) & 1) << p for p in range(5))
            pixels.append(tuple(palette[index])+(255,))
    out = Image.new("RGBA", (w, h))
    out.putdata(pixels)
    return out


def init():
    for folder in ("sources", "sheets", "art", "masters"):
        (ROOT/folder).mkdir(parents=True, exist_ok=True)
    entries, inputs, unresolved = [], {}, []
    for source in sorted(Path("workplace/amiga").glob("*.32")):
        parsed = amiga32.images(source.read_bytes())
        if parsed is None:
            unresolved.append(source.name)
            continue
        info, images = parsed
        inputs[source.name] = sha(source)
        for index, entry in enumerate(images):
            im = decode(entry, info["colors"])
            name = "%s-%03d" % (source.stem, index)
            im.save(ROOT/"sources"/(name+".png"))
            entries.append({"id": name, "source": source.name, "source_sha256": inputs[source.name], "index": index, "width": im.width, "height": im.height, "kind": "scene", "family": source.stem})
    pack_root = Path("workplace/amiga-monsters")
    pack = json.loads((pack_root/"set.json").read_text())
    for pic in pack["pictures"]:
        for index, filename in enumerate(pic["frames"]):
            source = pack_root/filename
            im = Image.open(source).convert("RGBA")
            # Contact sheets have a black backdrop; alpha is enforced from source at runtime.
            back = Image.new("RGBA", im.size, (0, 0, 0, 255))
            back.alpha_composite(im)
            name = "monster-"+source.stem
            back.save(ROOT/"sources"/(name+".png"))
            entries.append({"id": name, "source": filename, "source_sha256": sha(source), "index": index, "slot": pic["slot"], "pic": pic["pic"], "width": im.width, "height": im.height, "kind": "monster", "family": "monsters"})
    # Identical decoded images share the same art, even across animations.
    unique, duplicates = {}, {}
    for entry in entries:
        im = Image.open(ROOT/"sources"/(entry["id"]+".png"))
        identity = hashlib.sha256(bytes(str(im.size), "ascii")+im.tobytes()).hexdigest()
        if identity in unique:
            duplicates[entry["id"]] = unique[identity]["id"]
        else:
            unique[identity] = entry
    groups = {}
    for entry in unique.values():
        groups.setdefault(entry["family"], []).append(entry)
    sheets = []
    label = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 18)
    for family, group in groups.items():
        for start in range(0, len(group), COLS*ROWS):
            sheet_id = "%s-%02d" % (family, start//(COLS*ROWS))
            sheet = Image.new("RGB", (CELL*COLS, CELL*ROWS), "black")
            draw = ImageDraw.Draw(sheet)
            cells = []
            for cell, entry in enumerate(group[start:start+COLS*ROWS]):
                im = Image.open(ROOT/"sources"/(entry["id"]+".png")).convert("RGB")
                factor = min(3, (CELL-32)/im.width, (CELL-48)/im.height)
                w, h = max(1, round(im.width*factor)), max(1, round(im.height*factor))
                x, y = cell%COLS*CELL+(CELL-w)//2, cell//COLS*CELL+32+(CELL-40-h)//2
                sheet.paste(im.resize((w, h), Image.Resampling.NEAREST), (x, y))
                draw.text((cell%COLS*CELL+12, cell//COLS*CELL+5), "%02d  %s" % (cell, entry["id"]), font=label, fill=(170, 170, 170))
                draw.rectangle((cell%COLS*CELL, cell//COLS*CELL, (cell%COLS+1)*CELL-1, (cell//COLS+1)*CELL-1), outline=(40, 40, 40))
                cells.append({"entry": entry["id"], "rect": [x, y, x+w, y+h]})
            path = ROOT/"sheets"/(sheet_id+".png")
            sheet.save(path)
            sheets.append({"id": sheet_id, "family": family, "path": str(path), "width": sheet.width, "height": sheet.height, "cells": cells})
    approval = Path("docs/spec/assets/amiga-hd-approved.png")
    approved = Image.open(approval).convert("RGBA")
    assert approved.size == (1536, 1024), approved.size
    frame = approved.crop((0, 64, 1536, 1024)).resize((960, 600), Image.Resampling.LANCZOS)
    texture = approved.crop((1000, 810, 1390, 925))
    holes = [[44, 40, 652, 375], [671, 42, 918, 375], [31, 394, 259, 437], [277, 394, 471, 437], [491, 394, 708, 437], [726, 394, 929, 437], [44, 451, 918, 562]]
    for box in holes:
        frame.paste(texture.resize((box[2]-box[0], box[3]-box[1]), Image.Resampling.LANCZOS), box[:2])
    frame.save(ROOT/"frame.png")
    outer = frame.copy()
    outer.paste(texture.resize((875, 523)), (44, 40))
    outer.save(ROOT/"frame-page.png")
    record = {"version": 1, "source": "Amiga", "scale": 3, "approval": {"path": str(approval), "sha256": sha(approval)}, "inputs": inputs, "entries": entries, "duplicates": duplicates, "sheets": sheets, "unresolved_containers": unresolved, "frame": {"path": "frame.png", "sha256": sha(ROOT/"frame.png"), "page": "frame-page.png", "page_sha256": sha(ROOT/"frame-page.png"), "holes": holes}}
    write(ROOT/"inventory.json", record)
    print(json.dumps({"entries": len(entries), "unique": len(unique), "duplicates": len(duplicates), "sheets": len(sheets), "unresolved": unresolved}))


def visible_box(im):
    if im.mode=="RGBA":
        back=Image.new("RGBA",im.size,(0,0,0,255));back.alpha_composite(im);rgb=back.convert("RGB")
    else:rgb=im.convert("RGB")
    r,g,b=rgb.split()
    return ImageChops.lighter(ImageChops.lighter(r,g),b).point(lambda value:255 if value>12 else 0).getbbox()


def unpack(sheet,im,entries):
    for cell_number,cell in enumerate(sheet["cells"]):
        entry=entries[cell["entry"]]
        original=Image.open(ROOT/"sources"/(entry["id"]+".png")).convert("RGBA")
        if entry["family"] in ("townt","cavet","castlet") and im.getextrema()[3][0]<255:
            x0,y0,x1,y1=cell["rect"]
            crop=im.crop((round(x0*im.width/sheet["width"]),round(y0*im.height/sheet["height"]),round(x1*im.width/sheet["width"]),round(y1*im.height/sheet["height"])))
            crop.resize((entry["width"]*3,entry["height"]*3),Image.Resampling.LANCZOS).save(ROOT/"art"/(entry["id"]+".png"))
            continue
        # Register each generated drawing to its source ink bounds. Labels and
        # dividers are outside the content area and never enter runtime assets.
        col,row=cell_number%COLS,cell_number//COLS
        box=(round((col*CELL+8)*im.width/sheet["width"]),round((row*CELL+32)*im.height/sheet["height"]),round(((col+1)*CELL-8)*im.width/sheet["width"]),round(((row+1)*CELL-8)*im.height/sheet["height"]))
        content=im.crop(box)
        painted,source=visible_box(content),visible_box(original)
        result=Image.new("RGBA",(entry["width"]*3,entry["height"]*3),(0,0,0,255))
        if painted and source:
            paint=content.crop(painted).resize(((source[2]-source[0])*3,(source[3]-source[1])*3),Image.Resampling.LANCZOS)
            result.paste(paint,(source[0]*3,source[1]*3))
        elif source:
            raise ValueError("generated drawing is blank: "+entry["id"])
        result.save(ROOT/"art"/(entry["id"]+".png"))


def ingest(sheet_id, generated,revision=1):
    record = json.loads((ROOT/"inventory.json").read_text())
    sheet = next(s for s in record["sheets"] if s["id"] == sheet_id)
    entries = {entry["id"]: entry for entry in record["entries"]}
    im = Image.open(generated).convert("RGBA")
    if abs(im.width/im.height-sheet["width"]/sheet["height"]) > .01:
        raise ValueError("generated sheet aspect ratio mismatch")
    stem=sheet_id if revision==1 else sheet_id+"-r%d"%revision
    master = ROOT/"masters"/(stem+".png")
    if master.exists():
        raise ValueError("master already exists; preserve it and use a new revision")
    shutil.copyfile(generated, master)
    unpack(sheet,im,entries)
    sheet["input_sha256"] = sha(Path(sheet["path"]))
    if "generated" in sheet:sheet.setdefault("history",[]).append(sheet["generated"])
    sheet["generated"] = {"path": str(master), "sha256": sha(master), "width": im.width, "height": im.height, "tool": "image_gen.imagegen", "input_path": sheet["path"], "layout_cells": copy.deepcopy(sheet["cells"])}
    prompt=ROOT/"masters"/(stem+".prompt.json")
    if prompt.exists():sheet["generated"]["prompt_sha256"]=sha(prompt)
    write(ROOT/"inventory.json",record)
    finalize(record)


def repack():
    record=json.loads((ROOT/"inventory.json").read_text())
    entries={entry["id"]:entry for entry in record["entries"]}
    for sheet in record["sheets"]:
        if "generated" in sheet:
            master=Path(sheet["generated"]["path"])
            assert sha(master)==sheet["generated"]["sha256"]
            layout=dict(sheet,cells=sheet["generated"].get("layout_cells",sheet["cells"]))
            unpack(layout,Image.open(master).convert("RGBA"),entries)
    apply_details(record)
    finalize(record)


def apply_details(record):
    entries={entry["id"]:entry for entry in record["entries"]}
    for entry_id,detail in record.get("details",{}).items():
        master=Path(detail["path"])
        assert sha(master)==detail["sha256"],master
        entry=entries[entry_id]
        Image.open(master).convert("RGBA").resize((entry["width"]*3,entry["height"]*3),Image.Resampling.LANCZOS).save(ROOT/"art"/(entry_id+".png"))


def detail(entry_id,generated,revision):
    record=json.loads((ROOT/"inventory.json").read_text())
    entry=next(e for e in record["entries"] if e["id"]==entry_id)
    stem=entry_id+"-detail-r%d"%revision
    master=ROOT/"masters"/(stem+".png")
    assert not master.exists(),master
    previous=sha(ROOT/"art"/(entry_id+".png"))
    shutil.copyfile(generated,master)
    item={"path":str(master),"sha256":sha(master),"tool":"image_gen.imagegen","previous_art_sha256":previous,"source_sha256":entry["source_sha256"]}
    prompt=ROOT/"masters"/(stem+".prompt.json")
    if prompt.exists():item["prompt_sha256"]=sha(prompt)
    details=record.setdefault("details",{})
    if entry_id in details:item["previous"]=details[entry_id]
    details[entry_id]=item
    apply_details(record)
    write(ROOT/"inventory.json",record)
    finalize(record)


def reflow(sheet_id=None):
    record=json.loads((ROOT/"inventory.json").read_text())
    entries={entry["id"]:entry for entry in record["entries"]}
    label=ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",18)
    changed=[]
    for sheet in record["sheets"]:
        if sheet_id is not None and sheet["id"]!=sheet_id:continue
        if sheet.get("layout_revision")==2:continue
        if sheet_id is None and "generated" in sheet and sheet["family"] not in ("townt","cavet","castlet"):continue
        if "generated" in sheet:
            sheet["generated"]["input_path"]=sheet["path"]
            sheet["generated"]["layout_cells"]=copy.deepcopy(sheet["cells"])
        canvas=Image.new("RGB",(CELL*COLS,CELL*ROWS),"black")
        draw=ImageDraw.Draw(canvas)
        cells=[]
        for cell,old in enumerate(sheet["cells"]):
            entry=entries[old["entry"]]
            im=Image.open(ROOT/"sources"/(entry["id"]+".png")).convert("RGB")
            factor=min((CELL-32)/im.width,(CELL-48)/im.height)
            w,h=max(1,round(im.width*factor)),max(1,round(im.height*factor))
            x,y=cell%COLS*CELL+(CELL-w)//2,cell//COLS*CELL+32+(CELL-40-h)//2
            canvas.paste(im.resize((w,h),Image.Resampling.NEAREST),(x,y))
            draw.text((cell%COLS*CELL+12,cell//COLS*CELL+5),"%02d  %s"%(cell,entry["id"]),font=label,fill=(170,170,170))
            draw.rectangle((cell%COLS*CELL,cell//COLS*CELL,(cell%COLS+1)*CELL-1,(cell//COLS+1)*CELL-1),outline=(40,40,40))
            cells.append({"entry":entry["id"],"rect":[x,y,x+w,y+h]})
        path=ROOT/"sheets"/(sheet["id"]+"-r2.png")
        assert not path.exists(),path
        canvas.save(path)
        sheet["path"],sheet["cells"],sheet["layout_revision"]=str(path),cells,2
        changed.append(sheet["id"])
    write(ROOT/"inventory.json",record)
    print(json.dumps({"reflowed":len(changed),"canvas":2048,"observed_generated_canvas":1254,"purpose":"fill contact-sheet cells to retain native display detail"}))


def finalize(record=None):
    if record is None:
        record=json.loads((ROOT/"inventory.json").read_text())
    exported=[]
    for entry in record["entries"]:
        art_id=record["duplicates"].get(entry["id"],entry["id"])
        path=ROOT/"art"/(art_id+".png")
        if path.exists():
            exported.append(dict(entry, art="art/"+path.name, art_sha256=sha(path)))
    manifest={"version":1,"source":"Amiga","scale":3,"frame":record["frame"],"entries":exported,"expected":len(record["entries"]),"generated":len(exported),"monster_manifest_sha256":sha(Path("workplace/amiga-monsters/set.json")),"approval_sha256":record["approval"]["sha256"]}
    write(ROOT/"hd.json",manifest)
    pending=[s["id"] for s in record["sheets"] if "generated" not in s]
    print(json.dumps({"generated":len(exported),"expected":len(record["entries"]),"pending_sheets":len(pending),"next":pending[:3]}))


if __name__ == "__main__":
    parser=argparse.ArgumentParser()
    parser.add_argument("action",choices=["init","ingest","finalize","repack","reflow","detail"])
    parser.add_argument("--entry")
    parser.add_argument("--sheet")
    parser.add_argument("--generated",type=Path)
    parser.add_argument("--revision",type=int,default=1)
    args=parser.parse_args()
    if args.action=="init":init()
    elif args.action=="ingest":ingest(args.sheet,args.generated,args.revision)
    elif args.action=="repack":repack()
    elif args.action=="reflow":reflow(args.sheet)
    elif args.action=="detail":detail(args.entry,args.generated,args.revision)
    else:finalize()
