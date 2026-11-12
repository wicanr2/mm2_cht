#!/usr/bin/env python3
"""封裝本機完整版使用的素材，不攜帶母圖、重複磁片與研究快取。"""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import sys

ROOT = Path('/src')
D = Path(sys.argv[1])

def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()

def copy(p, rel):
    if not p.is_file():
        raise ValueError(f'missing runtime asset: {p}')
    target = D / rel
    target.parent.mkdir(parents=True, exist_ok=True)
    if target.exists():
        target.unlink()
    shutil.copy2(p, target)

# An existing local stage can refresh external resources without relinking.
# Require identical production sources and original binary bytes first.
prior = D/'SOURCE-MANIFEST.json'
if prior.exists():
    original = json.loads(prior.read_text())
    assert (D/'VERSION').read_text() == (ROOT/'VERSION').read_text()
    for rel, expected in original['files'].items():
        if rel.startswith(('cmd/','internal/','vendor/')) and not rel.endswith('_test.go'):
            assert sha(ROOT/rel) == expected, f'production source changed; rebuild required: {rel}'
    manifest = json.loads((D/'ASSET-MANIFEST.json').read_text())
    for rel, expected in manifest['files'].items():
        if rel.startswith('bin/'):
            assert sha(D/rel) == expected, f'binary changed: {rel}'
for rel in ['README.md','translations/display-build.json',
            'assets/font/toolbar-sans24.bin','assets/font/toolbar-sans30.bin','assets/font/toolbar-sans36.bin',
            'assets/font/display-sans24.bin','assets/font/display-sans30.bin',
            'assets/font/display-sans36.bin','assets/font/display-serif24.bin']:
    copy(ROOT/rel, rel)
for name in ['amiga-hd-overview.webp','amiga-hd-complete.webp']:
    copy(ROOT/'docs/gallery'/name, 'docs/gallery/'+name)

hd = ROOT/'workplace/hd-amiga-r1'
mf = json.loads((hd/'hd.json').read_text())
assert mf['expected'] == mf['generated'] == len(mf['entries'])
for name in sorted({e['source'] for e in mf['entries'] if e['kind']=='scene'}):
    copy(ROOT/'workplace/amiga'/name, 'workplace/amiga/'+name)
for name in ['hd.json', mf['frame']['path'], mf['frame']['page']]+sorted({e['art'] for e in mf['entries']}):
    copy(hd/name, 'workplace/hd-amiga-r1/'+name)
for name in ['set.json']+sorted({e['source'] for e in mf['entries'] if e['kind']=='monster'}):
    copy(ROOT/'workplace/amiga-monsters'/name, 'workplace/amiga-monsters/'+name)
for p in sorted((ROOT/'workplace/msx').glob('*.dsk')):
    if '[a]' not in p.name:
        copy(p, 'workplace/msx/'+p.name)
for folder in ['md-monsters','md-scene','modern','md-flavor']:
    src = ROOT/'workplace'/folder
    if not src.is_dir():
        continue
    for p in sorted(src.rglob('*')):
        if p.is_file() and p.suffix.lower() in {'.png','.json','.bin'}:
            copy(p, 'workplace/'+folder+'/'+p.relative_to(src).as_posix())
for p in sorted((ROOT/'vendor').rglob('*')):
    if p.is_file() and p.name.upper().startswith(('LICENSE','COPYING','NOTICE')):
        copy(p, 'THIRD-PARTY/'+p.relative_to(ROOT/'vendor').as_posix())
paths = subprocess.check_output(['git','-C',str(ROOT),'ls-files','--cached','--others','--exclude-standard','-z']).decode().split('\0')
sources = {}
for rel in sorted(set(paths)):
    p = ROOT/rel
    if p.is_file() and (rel.startswith(('cmd/','internal/','tools/','vendor/','translations/','assets/')) or rel in {'VERSION','go.mod','go.sum','LICENSE'}):
        sources[rel] = sha(p)
(D/'SOURCE-MANIFEST.json').write_text(json.dumps({'version':(ROOT/'VERSION').read_text().strip(),'commit':subprocess.check_output(['git','-C',str(ROOT),'rev-parse','HEAD']).decode().strip(),'files':sources},ensure_ascii=False,indent=2)+'\n')
runtime = {p.relative_to(D).as_posix():sha(p) for p in sorted(D.rglob('*')) if p.is_file() and p.name!='ASSET-MANIFEST.json'}
(D/'ASSET-MANIFEST.json').write_text(json.dumps({'version':(ROOT/'VERSION').read_text().strip(),'rights':'local-only; includes original game resources and derivative HD artwork','hd_entries':mf['generated'],'files':runtime},ensure_ascii=False,indent=2)+'\n')
print(f'[assets] {len(runtime)} files, {mf["generated"]} HD entries')
