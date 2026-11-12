#!/usr/bin/env python3
"""Capture delivered APK framebuffer frames around real touch actions."""
from pathlib import Path
import hashlib,json,os,subprocess,time,sys
sys.path.insert(0,'/tmp')
from android_smoke import adb,wait,state,action,dismiss_onboarding

OUT=Path('/out/android')
APK=Path('/src/dist-all/v.1.2.0-20261111/full-local/mm2-cht-android-local-full-v.1.2.0-20261111.apk')
clips=[];events=[];active_capture=None

def sha(p):return hashlib.file_digest(p.open('rb'),'sha256').hexdigest()
def step(identifier):
    s=action(identifier);events.append({'action':identifier,'state':s,'wall_time':time.time()})
    if active_capture:
        active_capture();time.sleep(.4);active_capture()
    print(identifier,s['mode'],s['overlay'],flush=True)
    return s
def explore():
    for _ in range(12):
        s=state()
        if s['mode']==0 and not s['overlay']:return
        ids=[t['id'] for t in s['targets']]
        step('confirm' if s['mode']==1 else ('back' if 'back' in ids else 'confirm'))
    raise RuntimeError('normal exploration not reached')
def more(identifier):
    step('more')
    for _ in range(4):
        if any(t['id']==identifier for t in state()['targets']):return step(identifier)
        step('next')
    raise RuntimeError(identifier)

def record(name,run):
    global active_capture
    folder=OUT/name;folder.mkdir(exist_ok=False)
    started=time.time();frames=[]
    def capture():
        assert time.time()-started<240, 'bounded capture duration exceeded'
        data=adb('exec-out','screencap','-p',timeout=30)
        assert data.startswith(b'\x89PNG\r\n\x1a\n') and len(data)>1000
        path=folder/('%05d.png'%len(frames));path.write_bytes(data)
        frames.append({'file':path.name,'seconds':time.time()-started,'sha256':sha(path)})
    active_capture=capture
    try:
        capture();time.sleep(1);capture()
        run()
        time.sleep(1)
        if time.time()-started<240:capture()
    finally:
        active_capture=None
    assert len(frames)>=3 and len({f['sha256'] for f in frames})>=2
    duration=time.time()-started
    (folder/'frames.json').write_text(json.dumps({'frames':frames,'seconds':duration,'method':'sequential actual frames at operation boundaries; timestamps retained'},indent=2)+'\n')
    clips.append({'path':name,'wall_seconds':duration,'events':list(events),'frames':frames})
    (OUT/'capture-progress.json').write_text(json.dumps(clips,ensure_ascii=False,indent=2)+'\n')
    events.clear();print('Recorded',name,len(frames),'real framebuffer frames',flush=True)

def main():
    wait(lambda:adb('shell','getprop','sys.boot_completed',check=False).strip()==b'1',900)
    print('Android booted',flush=True)
    for ns,key,val in [('global','window_animation_scale','0'),('global','transition_animation_scale','0'),('global','animator_duration_scale','0'),('system','accelerometer_rotation','0'),('system','user_rotation','1'),('system','show_touches','1')]:adb('shell','settings','put',ns,key,val)
    adb('shell','input','keyevent','82')
    # Acknowledge the OS-only first-use overlay before filming the game.
    adb('shell','settings','put','secure','immersive_mode_confirmations','confirmed')
    adb('install','-r',APK,timeout=180)
    adb('logcat','-c');adb('shell','am','start','-W','-n','com.anr2.mm2/.MainActivity',timeout=120)
    wait(lambda:(s:=state()) and s.get('frame_serial',0)>0 and min(s['viewport_dp'])>0,240)
    dismiss_onboarding()
    (OUT/'first-launch.png').write_bytes(adb('exec-out','screencap','-p'))
    if state()['mode']==10:step('confirm')
    explore()
    def walking():
        step('right');step('left');step('forward');explore();step('right');step('left')
    record('01-explore',walking)
    def menus():
        step('party:0');step('details');step('back');more('locale');step('map');step('back')
    record('02-controls',menus)
    more('create');step('classes')
    step(next(t['id'] for t in state()['targets'] if t['id'].startswith('class:')))
    for _ in range(3):step('menu:0')
    wait(lambda:state()['mode']==7,90)
    time.sleep(3)
    def editor():
        # Stock Android Latin keyboard, no synthetic pinyin/Chinese keyboard.
        adb('shell','input','text','CRON');time.sleep(2);active_capture()
        adb('shell','input','keyevent','4');time.sleep(2);active_capture()
    record('03-native-input',editor)
    result={'apk_sha256':sha(APK),'version':'v.1.2.0-20261111','hardware':'AOSP API23 x86 software emulator, no physical phone','capture':'real Android framebuffer frames at operation boundaries, timestamps retained; real launcher and touch events; accelerated during editing','input':'CRON entered using Android key input; no claim of a pinyin IME demonstration','clips':clips,'rights':'local only; original-derived game art and music'}
    (OUT/'PROVENANCE.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
    print('Android recording complete',flush=True)

if __name__=='__main__':
    try:main()
    finally:(OUT/'logcat.txt').write_bytes(adb('logcat','-d',check=False))
