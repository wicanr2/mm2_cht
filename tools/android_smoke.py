#!/usr/bin/env python3
"""容器內 Android 正常入口抽測；無傳送、發物品或修改遊戲狀態。"""
from pathlib import Path
import hashlib,json,subprocess,time,os,shutil,re,xml.etree.ElementTree as ET

OUT=Path(os.environ.get('ANDROID_SMOKE_OUT','/out/emulator'));OUT.mkdir(exist_ok=True)
ADB='/opt/android-sdk/platform-tools/adb'

def adb(*args,timeout=35,check=True):
    p=subprocess.run([ADB,*map(str,args)],capture_output=True,timeout=timeout)
    if check and p.returncode:raise RuntimeError(p.stderr.decode(errors='replace'))
    return p.stdout

def wait(predicate,seconds=60):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        try:result=predicate()
        except subprocess.TimeoutExpired:result=None
        if result:return result
        time.sleep(2)
    raise TimeoutError('Android condition timeout')

def state():
    raw=adb('logcat','-d','-s','MM2_STATE:I','*:S').decode(errors='replace')
    for line in reversed(raw.splitlines()):
        if '{' in line:
            try:return json.loads(line[line.index('{'):])
            except ValueError:pass
    return None

def screenshot(name):
    (OUT/(name+'.png')).write_bytes(adb('exec-out','screencap','-p'))

def tap(x,y):
    # Use the actual available dp, including any visible system navigation bar.
    current=state();w,h=current['viewport_dp'];scale=min(w/844,h/390)
    px=int(((w-844*scale)/2+x*scale)*2);py=int(((h-390*scale)/2+y*scale)*2)
    adb('shell','input','swipe',px,py,px,py,160)
    time.sleep(2)

def action(identifier):
    current=state();target=next(t for t in current['targets'] if t['id']==identifier)
    x1,y1,x2,y2=target['box'];tap((x1+x2)/2,(y1+y2)/2)
    return wait(lambda:(s:=state()) and s['action_serial']>current['action_serial'] and s['last_action']==identifier and s,90)

def dismiss_onboarding():
    if b'confirmed' in adb('shell','settings','get','secure','immersive_mode_confirmations',check=False):
        return
    adb('shell','uiautomator','dump','/data/local/tmp/mm2-window.xml',timeout=45,check=False)
    raw=adb('shell','cat','/data/local/tmp/mm2-window.xml',check=False)
    (OUT/'initial-window.xml').write_bytes(raw)
    try:tree=ET.fromstring(raw)
    except ET.ParseError:return
    for node in tree.iter('node'):
        if node.attrib.get('text','').upper()=='GOT IT':
            numbers=list(map(int,re.findall(r'\d+',node.attrib['bounds'])))
            x,y=(numbers[0]+numbers[2])//2,(numbers[1]+numbers[3])//2
            adb('shell','input','tap',x,y);time.sleep(2);return

def main():
    wait(lambda:adb('shell','getprop','sys.boot_completed',check=False).strip()==b'1',900)
    for ns,key,value in [('global','window_animation_scale','0'),('global','transition_animation_scale','0'),('global','animator_duration_scale','0'),('system','accelerometer_rotation','0'),('system','user_rotation','1')]:adb('shell','settings','put',ns,key,value)
    adb('shell','input','keyevent','82')
    required='lib/x86/libgojni.so' if os.environ.get('ANDROID_SMOKE_API')=='23' else 'lib/x86_64/libgojni.so'
    wait(lambda:Path('/out/apk-check.json').is_file() and required in json.loads(Path('/out/apk-check.json').read_text())['abis'],600)
    receipt=json.loads(Path('/out/apk-check.json').read_text());apk=Path('/tmp/mm2-smoke.apk');shutil.copy2('/out/mm2-android-local-full.apk',apk)
    assert hashlib.sha256(apk.read_bytes()).hexdigest()==receipt['sha256']
    adb('install','-r',apk,timeout=180)
    adb('logcat','-c')
    adb('shell','am','start','-W','-n','com.anr2.mm2/.MainActivity',timeout=120)
    first=wait(lambda:(s:=state()) and s.get('frame_serial',0)>0 and min(s['viewport_dp'])>0 and s,240);dismiss_onboarding();screenshot('01-start')
    if first['mode']==10:
        action('confirm');wait(lambda:(s:=state()) and s['mode']==0,60)
    start=state();screenshot('02-explore')
    turned=action('right');assert turned['face']!=start['face']
    screenshot('03-turn')
    restored=action('left');assert restored['face']==start['face']
    attempts=[];moved=None
    for _ in range(4):
        current=action('forward');attempts.append(current)
        if (current['map'],current['x'],current['y'])!=(start['map'],start['x'],start['y']):
            moved=current;break
        for _ in range(8):
            if state()['mode']==0:break
            ids=[t['id'] for t in state()['targets']]
            action('no' if 'no' in ids else ('confirm' if 'confirm' in ids else 'back'))
        action('right')
    assert moved,'no accessible adjacent square reached by normal touch input'
    screenshot('04-move')
    # Persist diagnostics without assuming that a facility uses the explore menu.
    (OUT/'initial-smoke.json').write_text(json.dumps({'apk_sha256':receipt['sha256'],'normal_entry':first,'start':start,'turn':turned,'move':moved,'attempts':attempts,'method':'normal launcher and touch input; no injected game state','hardware':'software Android API'+os.environ.get('ANDROID_SMOKE_API','35')+' emulator; not a physical phone'},indent=2)+'\n')
    print('Initial Android touch smoke passed',flush=True)

if __name__=='__main__':
    try:main()
    finally:
        (OUT/'logcat.txt').write_bytes(adb('logcat','-d',check=False))
