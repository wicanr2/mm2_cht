#!/usr/bin/env python3
"""Archive the existing commit-stamped outputs without relabeling builds."""
from pathlib import Path
import hashlib,json,os,zipfile

REV='ffa373aa81e7'
COMMIT='ffa373aa81e7ecdc2275c8ef80616092958cb035'
def sha(p):return hashlib.file_digest(p.open('rb'),'sha256').hexdigest()

def main():
    assert Path('/.dockerenv').exists() and os.getuid()!=0
    root=Path('/deliver');archive=root/REV
    if archive.exists():
        assert not any((root/n).exists() for n in ['local-full','public','promo'])
        data=json.loads((archive/'MIGRATION.json').read_text())
        for e in data['moved']:assert sha(archive/e['to'])==e['sha256']
        print('Legacy archive already verified');return
    mapping={'local-full':'full-local','public':'patch','promo':'promo'};moved=[]
    for old,new in mapping.items():
        directory=root/old;assert directory.is_dir() and directory.stat().st_uid==os.getuid()
        for p in directory.iterdir():
            assert p.is_file() and p.stat().st_uid==os.getuid()
            if old!='promo':assert REV in p.name
            if p.suffix=='.zip':
                with zipfile.ZipFile(p) as z:
                    n=next(n for n in z.namelist() if n.endswith('/PACKAGE-MANIFEST.txt'))
                    assert 'Commit: '+COMMIT in z.read(n).decode()
            moved.append({'from':old+'/'+p.name,'to':new+'/'+p.name,'bytes':p.stat().st_size,'sha256':sha(p)})
    assert len(moved)==8
    archive.mkdir()
    for old,new in mapping.items():(root/old).rename(archive/new)
    (root/'SHA256SUMS.txt').rename(archive/'SHA256SUMS.original.txt')
    (root/'README.md').rename(archive/'README.original.md')
    for e in moved:assert sha(archive/e['to'])==e['sha256']
    meta={'source_revision':REV,'source_commit':COMMIT,'formal_version':None,
          'classification':'historical pre-versioning archive; not a newly numbered release',
          'rights':'full-local and promo remain local only; patch rights unchanged','moved':moved}
    (archive/'MIGRATION.json').write_text(json.dumps(meta,indent=2)+'\n')
    (archive/'SHA256SUMS.txt').write_text(''.join(e['sha256']+'  '+e['to']+'\n' for e in sorted(moved,key=lambda x:x['to'])))
    (archive/'README.md').write_text('# 歷史 commit 版 '+REV+'\n\n這批封包形成於正式 version-date 契約之前，包內版本識別為來源 commit。\n原始檔名、程式內容與雜湊均保留；本次只搬移位置，沒有補造正式版號或重新發行。\n\n完整版在 `full-local/`，舊引擎包在 `patch/`，歷史影片在 `promo/`。\n原索引與校驗資料原樣保存於 `README.original.md` 與 `SHA256SUMS.original.txt`；\n搬移對照與目前路徑見 `MIGRATION.json` 和 `SHA256SUMS.txt`。\n\n現行交付回到[版本索引](../README.md)。完整包與影片仍只留本機。\n')
    print(json.dumps({'archive':str(archive),'verified_files':len(moved),'bytes_unchanged':True},indent=2))

if __name__=='__main__':main()
