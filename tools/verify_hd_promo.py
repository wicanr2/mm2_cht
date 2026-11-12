#!/usr/bin/env python3
"""驗證本輪推廣影片並保留逐幕截圖、來源與影音收據。容器內執行。"""
import hashlib
from fractions import Fraction
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys

movie,work=map(Path,sys.argv[1:3]);out=movie.parent
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
probe=json.loads(subprocess.check_output(['ffprobe','-v','error','-show_streams','-show_format','-of','json',str(movie)]))
video=next(s for s in probe['streams'] if s['codec_type']=='video')
audio=next(s for s in probe['streams'] if s['codec_type']=='audio')
assert (video['width'],video['height'])==(1920,1080)
assert abs(float(Fraction(video['avg_frame_rate']))-30)<.01
assert int(video['nb_frames'])==2160
assert abs(float(probe['format']['duration'])-72)<.1
assert audio['codec_name']=='aac' and int(audio['sample_rate'])==48000 and audio['channels']==2
command=['ffmpeg','-nostdin','-hide_banner','-threads','2','-filter_threads','2','-i',str(movie),'-vf','crop=1374:916:44:110,blackdetect=d=0.25:pix_th=0.05:pic_th=0.98,freezedetect=n=-45dB:d=3','-af','ebur128=peak=true,silencedetect=n=-50dB:d=2','-f','null','-']
run=subprocess.run(command,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,check=True)
log=run.stderr
(out/'video-analysis.log').write_text(log)
assert 'black_start:' not in log,'unexpected black game viewport'
peak=float(re.findall(r'Peak:\s*([-\d.]+) dBFS',log)[-1])
integrated=float(re.findall(r'I:\s*([-\d.]+) LUFS',log)[-1])
assert peak<-.5 and -30<integrated<-10,(peak,integrated)
frames=out/'frames';frames.mkdir(exist_ok=True)
times=[5,15,25,35,44,52,62,70]
for index,t in enumerate(times):
 subprocess.run(['ffmpeg','-nostdin','-v','error','-threads','2','-y','-ss',str(t),'-i',str(movie),'-frames:v','1',str(frames/f'{index:02}-{t:02}s.png')],check=True)
from PIL import Image,ImageDraw
sheet=Image.new('RGB',(1920,588),(10,18,31));d=ImageDraw.Draw(sheet)
for index,t in enumerate(times):
 im=Image.open(frames/f'{index:02}-{t:02}s.png').convert('RGB').resize((480,270))
 x,y=index%4*480,index//4*294
 sheet.paste(im,(x,y+24));d.text((x+8,y+5),f'{index+1:02} / 00:{t:02}',fill='white')
sheet.save(out/'contact-sheet.jpg',quality=95)
clips=['capture-r3/00-intro.mkv','capture-r3/01-explore.mkv','capture-r3/02-toolbar.mkv','capture-r3/03-facility.mkv','capture-r3/04-map.mkv','input.mkv','battle.mkv']
sources={p:sha(work/p) for p in clips}
for name in ['02-preedit.png','03-preedit-second.png','04-ime-confirm.png']:
 sources['input-gui/'+name]=sha(work/'input-gui'/name)
music=Path('/src/workplace/genesis/music')
tracks=['md-0B8AE0.wav','md-0B48D4.wav','md-0B2290.wav','md-0B8224.wav']
receipt={'version':Path('/src/VERSION').read_text().strip(),'movie':movie.name,'sha256':sha(movie),'duration':float(probe['format']['duration']),'resolution':[1920,1080],'fps':30,'audio':{'codec':'aac','sample_rate':48000,'channels':2,'integrated_lufs':integrated,'true_peak_dbfs':peak,'source':'original Mega Drive emulated captures','source_manifest_sha256':sha(music/'manifest.txt'),'tracks':{name:sha(music/name) for name in tracks},'listening':'human listening not certified'},'video_decode':'passed','black_game_viewport_intervals':[],'freeze_observations':[line for line in log.splitlines() if 'freeze_' in line],'static_holds':'UI menus, maps and the four IME screenshots intentionally hold. No claim of continuous live capture for these scenes.','sources':sources,'battle_fixture':{'seed':'0x1111','bestiary_indices':[3,4,5],'renderer':'cmd/mm2promo; same UI and rules; not a normal player path receipt'},'rights':'local-only; no public distribution authorization','tools':{'ffmpeg':subprocess.check_output(['ffmpeg','-version']).decode().splitlines()[0]},'caption_layout':{'game':[44,110,1374,916],'sidebar':[1460,110,416,918],'crop':'none; full game aspect ratio preserved'}}
(out/'PROVENANCE.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+'\n')
shutil.copyfile(work/'frames/storyboard.json',out/'storyboard.json')
print(json.dumps({k:receipt[k] for k in ['duration','resolution','fps','audio','video_decode','black_game_viewport_intervals','freeze_observations']},ensure_ascii=False,indent=2))
