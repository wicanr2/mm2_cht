#!/usr/bin/env python3
"""Compose and verify a 72-second desktop/Android trailer inside Docker."""
from pathlib import Path
import argparse,hashlib,json,re,subprocess
from fractions import Fraction
from PIL import Image,ImageDraw,ImageFont

def sha(p):return hashlib.file_digest(p.open('rb'),'sha256').hexdigest()
def run(args,**kw):subprocess.run(list(map(str,args)),check=True,**kw)
def probe(p):return json.loads(subprocess.check_output(['ffprobe','-v','error','-show_streams','-show_format','-of','json',str(p)]))
def font(size,bold=False):return ImageFont.truetype('/fonts/NotoSerifCJK-'+('Bold' if bold else 'Regular')+'.ttc',size,index=3)

def overlay(path,index,title,lines,kind,tag):
    im=Image.new('RGBA',(1920,1080),(15,23,35,255));d=ImageDraw.Draw(im);bounds=[]
    def text(x,y,value,size,color='#ead6ab',bold=False):
        f=font(size,bold);r=d.textbbox((x,y),value,font=f)
        assert 28<=r[0] and r[2]<=1892 and 16<=r[1] and r[3]<=1064,(value,r)
        d.text((x,y),value,font=f,fill=color);bounds.append({'text':value,'box':list(r)})
    d.rectangle((32,14,1888,17),fill='#ac9059')
    d.polygon([(47,43),(57,58),(47,73),(37,58)],fill='#a42d46',outline='#d2b77a')
    text(76,27,'魔法門 II',42,bold=True);text(1250,38,tag,22,color='#b3bece')
    if kind=='close':
        text(310,248,'冒險，從這裡繼續',78,bold=True)
        text(350,452,'Linux · Windows · macOS · Android',47)
        text(460,592,'桌面完整版 ＋ Android 本機測試版',41)
        text(570,769,'手繪 HD・繁簡中文・原生輸入',32,color='#bac6d6')
        text(550,986,'本機保存版本・含原版衍生素材',26,color='#a9b7c9')
        area=None
    elif kind.startswith('android'):
        area=(158,150,1604,780)
        text(50,92,title,31,bold=True)
        text(158,966,'　'.join(lines),29)
        note='Android 原生文字欄實測截圖・編輯前後畫面' if kind=='android-still' else 'Android 軟體模擬器實際畫面擷取・加速剪輯'
        text(158,1021,note,23,color='#a9b7c9')
    else:
        area=(44,130,1320,880)
        d.rounded_rectangle((1410,130,1876,1010),radius=12,fill='#192536',outline='#604653',width=2)
        text(1440,164,f'{index+1:02d} / 09',25,color='#c7a25d')
        text(1440,263,title,33,bold=True)
        d.rectangle((1440,336,1838,339),fill='#96394a')
        for n,line in enumerate(lines):text(1440,395+n*89,line,30)
        text(44,1024,'Amiga 手繪 HD・繁簡介面・完整素材',23,color='#a9b7c9')
    if area:
        x,y,w,h=area
        d.rectangle((x-3,y-3,x+w+2,y+h+2),outline='#aa8c55',width=2)
        d.rectangle((x,y,x+w-1,y+h-1),fill=(0,0,0,0))
    im.save(path)
    return {'game':list(area) if area else None,'caption_boxes':bounds}

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--work',type=Path,default=Path('/out'));ap.add_argument('--verify-existing',action='store_true')
    ap.add_argument('--version',default=Path('/src/VERSION').read_text().strip())
    ap.add_argument('--desktop-version');ap.add_argument('--android-source',type=Path)
    ap.add_argument('--workspace',default='workplace/promo-android-v.1.2.0-20261111')
    args=ap.parse_args();w=args.work;version=args.version;desktop_version=args.desktop_version or version
    assert Path('/.dockerenv').exists()
    android_source=args.android_source or w/'android'
    raw=json.loads((android_source/'PROVENANCE.json').read_text())
    assert raw['apk_sha256']=='09501c08be852c0c806de319aea105a19c6b7b677858712b1feaa33e30dfb926'
    if args.verify_existing:
        verify(w/f'mm2-cht-hd-android-promo-{version}.mp4',w,json.loads((w/'storyboard.json').read_text()),raw,Path('/src/dist-all/v.1.1.0-20261111/promo/mm2-cht-hd-promo-v.1.1.0-20261111.mp4'),version,desktop_version,args.workspace)
        return
    rendered=w/'edited';rendered.mkdir(exist_ok=True);frames=w/'frames';frames.mkdir(exist_ok=True)
    (w/'android').mkdir(exist_ok=True)
    for clip in raw['clips']:
        minimum=2 if clip.get('source_type')=='verified actual stills from same APK' else 3
        assert len({e['sha256'] for e in clip['frames']})>=minimum, 'capture has insufficient actual screen changes'
        folder=android_source/clip['path'];listing=w/'android'/(clip['path']+'.ffconcat')
        with listing.open('w') as f:
            f.write('ffconcat version 1.0\n')
            for i,entry in enumerate(clip['frames']):
                image=folder/entry['file'];assert sha(image)==entry['sha256']
                duration=clip['frames'][i+1]['seconds']-entry['seconds'] if i+1<len(clip['frames']) else 2
                f.write("file '"+str(image)+"'\nduration "+str(max(.05,duration))+'\n')
            f.write("file '"+str(folder/clip['frames'][-1]['file'])+"'\n")
        run(['ffmpeg','-nostdin','-v','error','-y','-safe','0','-f','concat','-i',listing,'-an','-fps_mode','vfr','-c:v','libx264','-preset','veryfast','-crf','18','-threads','2',w/'android'/(clip['path']+'.mkv')])
    previous=w if (w/'battle.mkv').exists() else Path('/src/workplace/release-v.1.1.0-20261111')
    scenes=[
        ('片頭重繪動畫',['熟悉的獨角獸','再次迎接冒險'],6,'desktop',w/'desktop/00-intro.mkv'),
        ('走入 Cron 世界',['Amiga 素材','手繪重製','石雕與古銀邊框'],8,'desktop',w/'desktop/01-explore.mkv'),
        ('畫面隨手調整',['繁體與簡體','原圖與 HD','工具列與字級'],8,'desktop',w/'desktop/02-toolbar.mkv'),
        ('回合制戰鬥',['選擇攻擊目標','手繪怪物動畫','戰鬥示範'],8,'desktop',previous/'battle.mkv'),
        ('Android・橫向雙手操作',['左側移動','右側情境指令','雕花按鍵'],12,'android',w/'android/01-explore.mkv'),
        ('Android・隊員與選單',['點選隊員','完整角色資料','繁簡切換與地圖'],12,'android',w/'android/02-controls.mkv'),
        ('Android・原生姓名輸入',['原生文字欄','繁簡姓名','確認與取消分開'],8,'android-still',w/'android/03-native-input.mkv'),
        ('中文輸入法',['繁簡中文姓名','組字與確認','Linux 實測畫面'],6,'desktop',previous/'input.mkv'),
        ('準備出發',[],4,'close',None),
    ]
    storyboard=[];offset=0
    for i,(title,lines,duration,kind,source) in enumerate(scenes):
        tag='Android v.1.2.0-20261111' if kind.startswith('android') else '桌面 '+desktop_version
        layout=overlay(frames/f'{i:02}.png',i,title,lines,kind,tag)
        output=rendered/f'{i:02}.mkv'
        if source:
            source_check=subprocess.run(['ffmpeg','-nostdin','-hide_banner','-threads','2','-filter_threads','2','-i',source,'-vf','blackdetect=d=0.5:pix_th=0.03:pic_th=0.98','-an','-f','null','-'],capture_output=True,text=True,check=True).stderr
            assert 'black_start:' not in source_check, 'black interval in actual source: '+str(source)
            info=probe(source);native=next(x for x in info['streams'] if x['codec_type']=='video')
            original=float(info['format']['duration']);factor=duration/original if kind=='android' else min(1,duration/original)
            x,y,width,height=layout['game']
            crop='crop=936:354:334:103,' if kind=='android-still' else ('crop=1604:780:0:0,' if kind=='android' else '')
            scale=f'scale={width}:{height}:flags=lanczos'
            if kind=='android-still':scale+=f':force_original_aspect_ratio=decrease,pad={width}:{height}:(ow-iw)/2:(oh-ih)/2:color=0x0f1723'
            vf=f'[0:v]{crop}setpts=(PTS-STARTPTS)*{factor:.9f},{scale},pad=1920:1080:{x}:{y}:color=0x0f1723[game];[game][1:v]overlay=0:0:shortest=1,tpad=stop_mode=clone:stop_duration=2,fps=30,setsar=1,format=yuv420p[v]'
            run(['ffmpeg','-nostdin','-v','error','-y','-filter_complex_threads','1','-i',source,'-loop','1','-i',frames/f'{i:02}.png','-filter_complex',vf,'-map','[v]','-an','-t',duration,'-r','30','-fps_mode','cfr','-c:v','libx264','-preset','veryfast','-crf','18','-threads','2',output])
            source_meta={'path':str(source),'sha256':sha(source),'duration':original,'playback_speed':1/factor,'original_dimensions':[native['width'],native['height']]}
        else:
            run(['ffmpeg','-nostdin','-v','error','-y','-loop','1','-i',frames/f'{i:02}.png','-an','-t',duration,'-r','30','-c:v','libx264','-preset','veryfast','-crf','18','-threads','2','-pix_fmt','yuv420p',output])
            source_meta=None
        storyboard.append({'scene':i,'start':offset,'duration':duration,'title':title,'lines':lines,'kind':kind,'layout':layout,'source':source_meta})
        offset+=duration;print('Rendered',i,title,flush=True)
    assert offset==72
    listing=rendered/'concat.txt';listing.write_text(''.join("file '"+str(rendered/f'{i:02}.mkv')+"'\n" for i in range(len(scenes))))
    run(['ffmpeg','-nostdin','-v','error','-y','-f','concat','-safe','0','-i',listing,'-c','copy',w/'silent.mkv'])
    audio=Path('/src/dist-all/v.1.1.0-20261111/promo/mm2-cht-hd-promo-v.1.1.0-20261111.mp4')
    movie=w/f'mm2-cht-hd-android-promo-{version}.mp4'
    # Finish the video encoder before muxing audio: -frames:v can otherwise
    # terminate audio copying before the delayed H.264 frames are flushed.
    video_only=w/'video-only.mp4'
    run(['ffmpeg','-nostdin','-v','error','-y','-filter_threads','1','-i',w/'silent.mkv','-vf','fps=30,tpad=stop_mode=clone:stop_duration=1','-an','-frames:v','2160','-r','30','-fps_mode','cfr','-video_track_timescale','30000','-c:v','libx264','-preset','veryfast','-crf','18','-threads','2',video_only])
    run(['ffmpeg','-nostdin','-v','error','-y','-i',video_only,'-i',audio,'-map','0:v:0','-map','1:a:0','-t','72','-c','copy','-movflags','+faststart','-metadata','title=魔法門 II HD 與 Android','-metadata','comment=Local-only desktop and Android capture. Android software-emulator frames accelerated. Original Mega Drive captured music.',movie])
    (w/'storyboard.json').write_text(json.dumps(storyboard,ensure_ascii=False,indent=2)+'\n')
    verify(movie,w,storyboard,raw,audio,version,desktop_version,args.workspace)

def verify(movie,w,storyboard,raw,audio,version,desktop_version,workspace):
    info=probe(movie);video=next(x for x in info['streams'] if x['codec_type']=='video');sound=next(x for x in info['streams'] if x['codec_type']=='audio')
    assert [video['width'],video['height']]==[1920,1080] and int(video['nb_frames'])==2160 and Fraction(video['avg_frame_rate'])==30
    assert abs(float(info['format']['duration'])-72)<.1 and sound['codec_name']=='aac' and sound['channels']==2
    assert abs(float(sound['duration'])-72)<.03
    def audio_hash(path):
        return subprocess.check_output(['ffmpeg','-nostdin','-v','error','-i',path,'-map','0:a:0','-c','copy','-f','hash','-hash','sha256','-'],text=True).strip()
    source_audio_hash=audio_hash(audio)
    assert source_audio_hash==audio_hash(movie), 'copied audio payload differs from source'
    (w/'audio-stream-check.json').write_text(json.dumps({'encoded_audio_payload_identical':True,'sha256':source_audio_hash,'source':str(audio),'movie':movie.name},indent=2)+'\n')
    (w/'ffprobe.json').write_text(json.dumps(info,indent=2)+'\n')
    analysis=subprocess.run(['ffmpeg','-nostdin','-hide_banner','-threads','2','-filter_threads','2','-i',movie,'-vf','blackdetect=d=0.4:pix_th=0.03:pic_th=0.98,freezedetect=n=-50dB:d=3','-af','ebur128=peak=true,silencedetect=n=-50dB:d=2','-f','null','-'],capture_output=True,text=True,check=True).stderr
    (w/'video-analysis.log').write_text(analysis)
    assert 'black_start:' not in analysis
    peak=float(re.findall(r'Peak:\s*([-\d.]+) dBFS',analysis)[-1]);lufs=float(re.findall(r'I:\s*([-\d.]+) LUFS',analysis)[-1])
    assert peak<-.5 and -30<lufs<-10 and 'silence_start:' not in analysis,(peak,lufs)
    samples=w/'samples';samples.mkdir(exist_ok=True)
    sheet=Image.new('RGB',(1920,1152),(15,23,35));d=ImageDraw.Draw(sheet)
    for i,scene in enumerate(storyboard):
        t=scene['start']+scene['duration']/2;file=samples/f'{i:02}-{t:g}s.png'
        run(['ffmpeg','-nostdin','-v','error','-y','-threads','2','-ss',t,'-i',movie,'-frames:v','1',file])
        shot=Image.open(file).convert('RGB');shot.thumbnail((640,360));x=i%3*640;y=i//3*384
        sheet.paste(shot,(x,y+24));d.text((x+12,y+5),f'{i+1:02} / {t:g}s',fill='white')
    sheet.save(w/'contact-sheet.jpg',quality=95)
    old=json.loads(Path('/src/dist-all/v.1.1.0-20261111/promo/PROVENANCE.json').read_text())
    receipt={'version':version,'movie':movie.name,'sha256':sha(movie),'duration':72,'resolution':[1920,1080],'fps':30,'frames':2160,
             'workspace_source_root':workspace,'container_paths':{'repository':'/src','working_output':'/out'},
             'recording_script':'tools/record_android_promo.py; code and sampling method recorded in Android provenance',
             'renderer_script_sha256':sha(Path(__file__)),
             'audio':{'source_movie':str(audio),'source_sha256':sha(audio),'source_provenance':old['audio'],'reuse':'AAC stream copied without alteration','integrated_lufs':lufs,'true_peak_dbfs':peak},
             'android':raw,'desktop_package':{'version':desktop_version,'capture':'fresh normal GUI recording of delivered AppImage'},'storyboard':storyboard,
             'battle_fixture':old['battle_fixture'],'freeze_observations':[s for s in analysis.splitlines() if 'freeze_' in s],
             'static_holds':'Turn-based menus, map, native editor, Linux IME still sequence, closing card. Android is sampled actual framebuffer capture accelerated; no claim of physical-phone performance.',
             'decode':'passed','black_intervals':[],'caption_bounds':'all declared text boxes within 1920x1080 safe margins','rights':'local only; original-derived artwork and original captured Mega Drive audio','capture_date':'2026-11-12 Asia/Taipei'}
    (w/'PROVENANCE.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'movie':str(movie),'sha256':sha(movie),'lufs':lufs,'peak':peak,'verified':True},indent=2))

if __name__=='__main__':main()
