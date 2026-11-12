#!/usr/bin/env python3
"""在鎖版 Android 容器內編譯本機完整版；只寫 /out 與容器暫存。"""
import argparse
import csv
import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile

ROOT = Path('/src')
SDK = Path('/opt/android-sdk')
BT = SDK/'build-tools/35.0.0'

def run(args, **kwargs):
    print('+', ' '.join(map(str,args)), flush=True)
    subprocess.run(list(map(str,args)),check=True,**kwargs)

def sha(path):
    with path.open('rb') as stream:
        digest=hashlib.file_digest(stream,'sha256')
    return digest.hexdigest()

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--reuse-aar',action='store_true');args=ap.parse_args()
    assert Path('/.dockerenv').exists() and os.getuid()!=0
    out=Path('/out');assert out.is_dir() and out.stat().st_uid==os.getuid()
    version=(ROOT/'VERSION').read_text().strip()
    assert re.fullmatch(r'v\.\d+\.\d+\.\d+-\d{8}',version),'invalid release version'
    native_inputs={n:sha(ROOT/n) for n in ['android/MainActivity.java','android/AndroidManifest.xml','tools/build_android.py']}
    sources={p.relative_to(ROOT).as_posix():sha(p) for folder in ['mobile','internal','vendor'] for p in sorted((ROOT/folder).rglob('*')) if p.is_file() and not p.name.endswith('_test.go')}
    sources.update({n:sha(ROOT/n) for n in ['go.mod','go.sum','VERSION']})
    targets='android/arm64,android/amd64,android/386'
    fingerprint=hashlib.sha256(json.dumps({'files':sources,'targets':targets},sort_keys=True).encode()).hexdigest()
    aar=out/'mm2.aar';receipt=out/'aar-source.json'
    if args.reuse_aar:
        assert aar.is_file() and json.loads(receipt.read_text())['fingerprint']==fingerprint,'AAR source changed; rebuild required'
    else:
        with tempfile.TemporaryDirectory(prefix='mm2-go-') as temporary:
            tmp=Path(temporary);src=tmp/'src';src.mkdir()
            for name in ['mobile','internal','vendor']:
                shutil.copytree(ROOT/name,src/name)
            for name in ['go.mod','go.sum']:shutil.copy2(ROOT/name,src/name)
            modules=tmp/'gomod'
            shutil.copytree('/module-seed',modules)
            for p in modules.rglob('*'):
                if not p.is_symlink():p.chmod(p.stat().st_mode|0o200)
            shutil.copytree(ROOT/'workplace/cache/gomod',modules,dirs_exist_ok=True)
            cache=out/'cache/go';cache.mkdir(parents=True,exist_ok=True)
            env=os.environ.copy();env.update(HOME=str(tmp/'home'),GOPATH=str(tmp/'gopath'),GOMODCACHE=str(modules),GOCACHE=str(cache),GOPROXY='off',GOSUMDB='off',GOFLAGS='-mod=readonly',GOWORK='off',GOTOOLCHAIN='local',GOMAXPROCS='2',CGO_LDFLAGS='-Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384')
            run(['ebitenmobile','bind','-trimpath','-ldflags=-buildid= -X github.com/wicanr2/mm2_cht/mobile.Version='+version,'-target',targets,'-androidapi','21','-javapkg','com.anr2.mm2','-o',aar,'./mobile'],cwd=src,env=env)
        receipt.write_text(json.dumps({'fingerprint':fingerprint,'files':sources,'targets':targets,'aar_sha256':sha(aar)},indent=2)+'\n')

    runtime_zip=out/'runtime.zip'
    current_assets={n:sha(ROOT/n) for n in ['translations/interface.tsv','translations/display-build.json']}
    runtime_meta=out/'runtime-source.json'
    cached=json.loads(runtime_meta.read_text()) if runtime_meta.exists() else {}
    if not runtime_zip.exists() or cached.get('version')!=version or cached.get('updated_assets')!=current_assets or 'workplace/orig/MM2/MAP.DAT' not in cached.get('files',{}) or 'THIRD-PARTY/Go-LICENSE.txt' not in cached.get('files',{}):
        with tempfile.TemporaryDirectory(prefix='mm2-assets-') as temporary:
            tmp=Path(temporary)
            base=ROOT/'dist-all/v.1.1.0-20261111/full-local/mm2-cht-linux-x64-local-full-v.1.1.0-20261111.AppImage'
            run([base,'--appimage-extract'],cwd=tmp,stdout=subprocess.DEVNULL)
            resource=tmp/'squashfs-root/usr'
            folders=['assets','translations','data','THIRD-PARTY','workplace/amiga','workplace/amiga-monsters','workplace/hd-amiga-r1','music']
            entries={p.relative_to(resource).as_posix():p for f in folders for p in (resource/f).rglob('*') if p.is_file()}
            entries['THIRD-PARTY/Go-LICENSE.txt']=Path('/usr/local/go/LICENSE')
            original=resource/'original-data'
            assert original.is_dir(),'desktop full package has no original-data directory'
            entries.update({'workplace/orig/MM2/'+p.relative_to(original).as_posix():p for p in original.rglob('*') if p.is_file()})
            for name in ['MM2.EXE','MAP.DAT','EVENTSI.DAT','ATTRIB.DAT','MM2.CH','DEFAULT.DAT','MONSTERS.DAT','ITEMS.DAT']:
                assert 'workplace/orig/MM2/'+name in entries,'missing mandatory original: '+name
            for n in ['translations/interface.tsv','translations/display-build.json']:
                entries[n]=ROOT/n
            for p in (ROOT/'assets/font').glob('*.bin'):entries[p.relative_to(ROOT).as_posix()]=p
            for n in ['LICENSE','NOTICE','VERSION']:
                if (ROOT/n).is_file():entries[n]=ROOT/n
            files={n:sha(p) for n,p in entries.items()}
            with zipfile.ZipFile(runtime_zip,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=6,strict_timestamps=False) as z:
                for n,p in sorted(entries.items()):z.write(p,n)
                z.writestr('ANDROID-ASSETS.json',json.dumps({'version':version,'rights':'local-only; original game resources and derivative artwork','files':files},indent=2))
            runtime_meta.write_text(json.dumps({'version':version,'updated_assets':current_assets,'runtime_sha256':sha(runtime_zip),'files':files},indent=2)+'\n')

    with tempfile.TemporaryDirectory(prefix='mm2-apk-') as temporary:
        tmp=Path(temporary);native=tmp/'aar';native.mkdir()
        with zipfile.ZipFile(aar) as z:z.extractall(native)
        labels={row['key']:row['text'] for row in csv.DictReader((ROOT/'translations/interface.tsv').open(),delimiter='\t') if row['locale']=='zh-Hant'}
        manifest=tmp/'AndroidManifest.xml';manifest.write_text((ROOT/'android/AndroidManifest.xml').read_text().replace('@APP_LABEL@',labels['mobile.title']))
        android_jar=SDK/'platforms/android-35/android.jar'
        base_apk=tmp/'base.apk'
        parts=version.split('-')[0].removeprefix('v.').split('.')
        code=int(parts[0])*1000000+int(parts[1])*1000+int(parts[2])
        run([BT/'aapt2','link','-I',android_jar,'--manifest',manifest,'--min-sdk-version','21','--target-sdk-version','35','--version-code',str(code),'--version-name',version,'-o',base_apk])
        classes=tmp/'classes';classes.mkdir()
        run(['javac','-encoding','UTF-8','-source','17','-target','17','-classpath',str(android_jar)+':'+str(native/'classes.jar'),'-d',classes,ROOT/'android/MainActivity.java'])
        jar=tmp/'app.jar';run(['jar','cf',jar,'-C',classes,'.'])
        dex=tmp/'dex';dex.mkdir()
        run([BT/'d8','--min-api','21','--lib',android_jar,'--output',dex,jar,native/'classes.jar'])
        with zipfile.ZipFile(base_apk,'a') as z:
            z.write(dex/'classes.dex','classes.dex',compress_type=zipfile.ZIP_DEFLATED)
            for p in sorted((native/'jni').rglob('*.so')):z.write(p,'lib/'+p.relative_to(native/'jni').as_posix(),compress_type=zipfile.ZIP_STORED)
            z.write(runtime_zip,'assets/runtime.zip',compress_type=zipfile.ZIP_STORED)
            z.writestr('assets/runtime.sha256',sha(runtime_zip)+'\n')
            for name in ['LICENSE','NOTICE','VERSION']:
                if (ROOT/name).exists():z.write(ROOT/name,'assets/'+name)
        key=out/'debug.keystore'
        if not key.exists():run(['keytool','-genkeypair','-keystore',key,'-storepass','android','-alias','androiddebugkey','-keypass','android','-dname','CN=Android Debug,O=Android,C=US','-keyalg','RSA','-keysize','2048','-validity','10000'])
        aligned=tmp/'aligned.apk';run([BT/'zipalign','-P','16','-f','4',base_apk,aligned])
        apk=tmp/'signed.apk'
        run([BT/'apksigner','sign','--ks',key,'--ks-pass','pass:android','--key-pass','pass:android','--out',apk,aligned])
        run([BT/'apksigner','verify','--verbose',apk])
        run([BT/'zipalign','-c','-P','16','4',apk])
        run([BT/'aapt','dump','badging',apk],stdout=(out/'apk-badging.txt').open('w'))
        with zipfile.ZipFile(apk) as z:
            assert z.testzip() is None
            abis=sorted(n for n in z.namelist() if n.startswith('lib/') and n.endswith('.so'))
            assert abis==['lib/arm64-v8a/libgojni.so','lib/x86/libgojni.so','lib/x86_64/libgojni.so'],abis
        candidate=out/'mm2-candidate.apk';shutil.copy2(apk,candidate)
        assert native_inputs=={n:sha(ROOT/n) for n in native_inputs},'native build sources changed during packaging'
        meta=json.dumps({'version':version,'sha256':sha(apk),'bytes':apk.stat().st_size,'abis':abis,'min_api':21,'target_api':35,'signing':'local debug key; not a store release','rights':'local only','zip_crc':'passed','zipalign_16kb':'passed','source_fingerprint':fingerprint,'native_inputs':native_inputs,'runtime_sha256':sha(runtime_zip)},indent=2)+'\n'
        (out/'apk-check.pending.json').write_text(meta)
        candidate.replace(out/'mm2-android-local-full.apk');(out/'apk-check.pending.json').replace(out/'apk-check.json')
        print('APK:',out/'mm2-android-local-full.apk',apk.stat().st_size,'bytes',flush=True)

if __name__=='__main__':main()
