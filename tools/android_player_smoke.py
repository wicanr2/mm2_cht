#!/usr/bin/env python3
"""正常觸控、原生 IME 與程序重啟抽測。測試輸入法不隨遊戲發行。"""
import json
import hashlib
import sys
import re
import time
import xml.etree.ElementTree as ET
from pathlib import Path
from android_smoke import adb, wait, state, screenshot, tap, OUT

events = []


def record(label):
    current = state()
    assert current and current['frame_serial'] > 0
    events.append({'step': label, 'state': current})
    (OUT/'player-progress.json').write_text(json.dumps(events, ensure_ascii=False, indent=2)+'\n')
    print(label, current['mode'], current['overlay'], flush=True)
    return current


def click(identifier):
    current = state()
    target = next(t for t in current['targets'] if t['id'] == identifier)
    assert min(target['width_dp'], target['height_dp']) >= 48, target
    x1, y1, x2, y2 = target['box']
    tap((x1+x2)/2, (y1+y2)/2)
    wait(lambda: (s := state()) and s['action_serial'] > current['action_serial'] and s['last_action'] == identifier, 90)
    return record(identifier)


def more(identifier):
    click('more')
    for _ in range(4):
        if any(t['id'] == identifier for t in state()['targets']):
            return click(identifier)
        click('next')
    raise AssertionError(identifier)


def explore():
    for _ in range(8):
        current = state()
        if current['mode'] == 0 and not current['overlay']:
            return current
        identifiers = [t['id'] for t in current['targets']]
        click('confirm' if current['mode'] == 1 and 'confirm' in identifiers else ('back' if 'back' in identifiers else 'confirm'))
    raise AssertionError('cannot return to exploration')


def tree():
    adb('shell', 'uiautomator', 'dump', '/data/local/tmp/mm2-editor.xml', timeout=45, check=False)
    raw = adb('shell', 'cat', '/data/local/tmp/mm2-editor.xml')
    return ET.fromstring(raw)


def native(label):
    if label.lower() in ['compose', 'delete', 'commit', 'enter']:
        # UiAutomator on API23 exposes only the focused application window.
        # This separate fixture has four equal columns and a 110dp input view.
        index = ['compose', 'delete', 'commit', 'enter'].index(label.lower())
        width, height = state()['viewport_dp']
        adb('shell', 'input', 'tap', int(width*2*(index+0.5)/4), int(height*2-110))
        time.sleep(2)
        return
    nodes = list(tree().iter('node'))
    node = next(n for n in nodes if n.attrib.get('text', '').lower() == label.lower())
    bounds = list(map(int, re.findall(r'\d+', node.attrib['bounds'])))
    x,y=(bounds[0]+bounds[2])//2,(bounds[1]+bounds[3])//2
    adb('shell', 'input', 'swipe', x, y, x, y, 160)
    time.sleep(2)
    if label == '取消':
        wait(lambda:'android.widget.EditText' not in ET.tostring(tree(),encoding='unicode'),60)


def field():
    return next(n.attrib.get('text', '') for n in tree().iter('node') if n.attrib.get('class') == 'android.widget.EditText')


def save():
    return json.loads(adb('shell', 'run-as', 'com.anr2.mm2', 'cat', 'files/player/save/state.json'))


def create():
    more('create')
    assert state()['mode'] == 6
    click('classes')
    identifier = next(t['id'] for t in state()['targets'] if t['id'].startswith('class:'))
    click(identifier)
    for _ in range(3):
        click('menu:0')
    wait(lambda: state()['mode'] == 7, 90)
    wait(lambda: 'android.widget.EditText' in ET.tostring(tree(), encoding='unicode'), 90)


def main():
    record('normal-player-start')
    explore()
    click('party:0'); click('details'); screenshot('05-character'); click('back')
    explore()
    click('items'); screenshot('06-inventory')
    click('next'); click('next'); click('menu:6'); click('confirm')
    explore()
    more('shop'); screenshot('07-shop'); click('menu:0'); screenshot('07-shop-stock'); explore()
    more('rest'); screenshot('07-rest'); explore()
    click('map'); assert state()['mode'] == 5; screenshot('08-local-map'); click('back')
    more('world'); assert state()['mode'] == 9; screenshot('09-world-map'); click('back')
    if state()['locale'] != 'zh-Hans':
        more('locale')
    assert state()['locale'] == 'zh-Hans'
    screenshot('10-simplified')

    adb('install', '-r', '/out/mm2-test-ime.apk', timeout=180)
    component = 'com.anr2.mm2testime/.ProtocolIME'
    adb('shell', 'ime', 'enable', component)
    adb('shell', 'ime', 'set', component)
    adb('shell', 'settings', 'put', 'secure', 'show_ime_with_hard_keyboard', '1')
    count = state()['roster_count']
    create()
    native('Compose'); assert field() == '测试long'
    native('Enter'); assert state()['mode'] == 7 and state()['roster_count'] == count
    screenshot('11-native-composition')
    native('Delete'); assert field() == '测试lon'
    native('Compose'); native('Commit'); assert field() == '测试long'
    assert state()['mode'] == 7 and state()['roster_count'] == count
    screenshot('12-native-commit')
    native('Enter')
    wait(lambda: state()['roster_count'] == count+1, 90)
    assert state()['mode'] == 1
    explore()
    checkpoint = save()
    assert '测试long' in json.dumps(checkpoint, ensure_ascii=False)
    screenshot('13-name-saved')
    finish(count, checkpoint)


def finish(count, checkpoint):
    (OUT/'before-cancel.json').write_text(json.dumps(checkpoint,ensure_ascii=False,indent=2)+'\n')

    create(); native('Compose'); native('取消')
    assert state()['mode'] == 7 and state()['roster_count'] == count+1
    click('back'); explore()
    after = save()
    (OUT/'after-cancel.json').write_text(json.dumps(after,ensure_ascii=False,indent=2)+'\n')
    before_semantics = {k:v for k,v in checkpoint.items() if k != 'seed'}
    after_semantics = {k:v for k,v in after.items() if k != 'seed'}
    assert before_semantics == after_semantics
    events.append({'step':'cancel-preserves-characters-and-progress', 'seed_before':checkpoint['seed'], 'seed_after':after['seed'], 'allowed_difference':'existing character roll advances the saved RNG seed'})
    checkpoint = after
    before_background = state()
    adb('shell', 'input', 'keyevent', '3')
    time.sleep(2)
    adb('shell', 'am', 'start', '-W', '-n', 'com.anr2.mm2/.MainActivity', timeout=120)
    wait(lambda: (s := state()) and not s['paused'] and s['frame_serial'] >= before_background['frame_serial']+2, 90)
    assert save() == checkpoint
    record('background-resume')

    adb('shell', 'am', 'force-stop', 'com.anr2.mm2')
    adb('logcat', '-c')
    adb('shell', 'am', 'start', '-W', '-n', 'com.anr2.mm2/.MainActivity', timeout=120)
    wait(lambda: (s := state()) and s['frame_serial'] > 0 and s['mode'] == 0, 240)
    restored = record('new-process-restored')
    assert restored['roster_count'] == count+1 and restored['locale'] == 'zh-Hans'
    assert save() == checkpoint
    screenshot('14-reloaded')
    result = {'apk_sha256': json.loads(Path('/out/apk-check.json').read_text())['sha256'],
              'test_ime_sha256': hashlib.sha256(Path('/out/mm2-test-ime.apk').read_bytes()).hexdigest(),
              'result': 'passed', 'events': events,
              'input': 'real Android InputConnection fixture; composing Enter, delete, commit, submit and cancel',
              'limits': 'API23 software emulator; third-party pinyin keyboards and physical phones not tested'}
    (OUT/'player-smoke.json').write_text(json.dumps(result, ensure_ascii=False, indent=2)+'\n')
    print('Android player and native input smoke passed', flush=True)


def items_smoke():
    explore()
    before = save()
    more('shop'); click('menu:0'); click('menu:0'); explore()
    purchased = save()
    old = before['characters']['party'][0]
    new = purchased['characters']['party'][0]
    assert new['Gold'] < old['Gold']
    assert sum(bool(x['ID']) for x in new['Items']) == sum(bool(x['ID']) for x in old['Items'])+1
    click('items'); click('next'); click('next'); click('menu:6'); click('use')
    screenshot('15-item-use')
    explore()
    result={'result':'passed', 'apk_sha256':json.loads(Path('/out/apk-check.json').read_text())['sha256'],
            'gold_before':old['Gold'], 'gold_after_purchase':new['Gold'], 'purchased_item':new['Items'][6],
            'use_scope':'normal inventory Use command; nonmagical weapon refusal, not charged item consumption',
            'events':events}
    (OUT/'items-smoke.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
    print('Normal purchase and inventory Use smoke passed',flush=True)


if __name__ == '__main__':
    try:
        if '--items' in sys.argv:
            items_smoke()
        elif '--resume-cancel' in sys.argv:
            events.extend(json.loads((OUT/'player-progress.json').read_text()))
            explore()
            finish(state()['roster_count']-1, save())
        else:
            main()
    finally:
        (OUT/'player-logcat.txt').write_bytes(adb('logcat', '-d', check=False))
