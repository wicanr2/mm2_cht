#!/usr/bin/env python3
"""建立獨立原生組字測試輸入法，只安裝於本機測試模擬器。"""
from pathlib import Path
import os,shutil,subprocess,tempfile,zipfile

assert Path('/.dockerenv').exists() and os.getuid()!=0
out=Path('/out');bt=Path('/opt/android-sdk/build-tools/35.0.0');jar=Path('/opt/android-sdk/platforms/android-35/android.jar')
def run(args):subprocess.run(list(map(str,args)),check=True)
with tempfile.TemporaryDirectory(prefix='mm2-ime-test-') as td:
    tmp=Path(td);res=tmp/'res/xml';res.mkdir(parents=True);shutil.copy2('/src/tools/android-test/method.xml',res/'method.xml')
    run([bt/'aapt2','compile','--dir',tmp/'res','-o',tmp/'resources.zip'])
    run([bt/'aapt2','link','-I',jar,'--manifest','/src/tools/android-test/AndroidManifest.xml','--min-sdk-version','21','--target-sdk-version','35','-o',tmp/'base.apk',tmp/'resources.zip'])
    classes=tmp/'classes';classes.mkdir();run(['javac','-encoding','UTF-8','-source','17','-target','17','-cp',jar,'-d',classes,'/src/tools/android-test/ProtocolIME.java'])
    run(['jar','cf',tmp/'classes.jar','-C',classes,'.']);dex=tmp/'dex';dex.mkdir();run([bt/'d8','--lib',jar,'--min-api','21','--output',dex,tmp/'classes.jar'])
    with zipfile.ZipFile(tmp/'base.apk','a') as z:z.write(dex/'classes.dex','classes.dex')
    run([bt/'zipalign','-f','4',tmp/'base.apk',tmp/'aligned.apk'])
    run([bt/'apksigner','sign','--ks',out/'debug.keystore','--ks-pass','pass:android','--key-pass','pass:android','--out',out/'mm2-test-ime.apk',tmp/'aligned.apk'])
    run([bt/'apksigner','verify',out/'mm2-test-ime.apk'])
print('Native IME protocol test APK ready; not part of the game.')
