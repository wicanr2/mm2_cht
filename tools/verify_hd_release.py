#!/usr/bin/env python3
"""容器內驗證最終三平台封包的內容、資源、版本與 Mach-O 簽章。"""
import hashlib
import json
from pathlib import Path
import plistlib
import struct
import sys
import zipfile

release, linux = map(Path, sys.argv[1:3])
version = (Path('/src')/'VERSION').read_text().strip()
result={'version':version,'platforms':{},'hardware_limits':['Windows uses Wine, not a native machine','macOS has structural validation only','Native Windows/macOS IME still needs hardware verification']}
shared=[]

def macho(data):
    magic,count=struct.unpack_from('>II',data)
    assert magic==0xcafebabe and count==2
    slices=[]
    for i in range(count):
        cpu,sub,off,size,align=struct.unpack_from('>IIIII',data,8+i*20)
        b=data[off:off+size]
        assert struct.unpack_from('<I',b)[0]==0xfeedfacf
        ncmds=struct.unpack_from('<I',b,16)[0]
        pos=32;sig=None;minimum=None
        for _ in range(ncmds):
            cmd,length=struct.unpack_from('<II',b,pos)
            if cmd==0x1d:sig=struct.unpack_from('<II',b,pos+8)
            if cmd==0x32:minimum=struct.unpack_from('<I',b,pos+12)[0]
            if cmd==0x24:minimum=struct.unpack_from('<I',b,pos+8)[0]
            pos+=length
        assert minimum is not None and minimum>=12<<16
        signed=False;pages=0
        if sig:
            blob=b[sig[0]:sig[0]+sig[1]]
            m,length,n=struct.unpack_from('>III',blob)
            assert m==0xfade0cc0
            for j in range(n):
                slot,cd_off=struct.unpack_from('>II',blob,12+8*j)
                if slot!=0:continue
                cd=blob[cd_off:]
                m,l,v,flags,hashoff,ident,special,slots,limit=struct.unpack_from('>9I',cd)
                hashsize,hashtype,platform,page=struct.unpack_from('>4B',cd,36)
                assert m==0xfade0c02 and flags&2 and hashtype==2 and hashsize==32
                for pageidx in range(slots):
                    actual=hashlib.sha256(b[pageidx*(1<<page):min((pageidx+1)*(1<<page),limit)]).digest()
                    assert actual==cd[hashoff+pageidx*hashsize:hashoff+(pageidx+1)*hashsize],f'bad signature page {pageidx}'
                signed=True;pages=slots
        if cpu==0x100000c:assert signed,'arm64 requires valid ad-hoc code signature'
        slices.append({'cpu':hex(cpu),'minimum_macos':f'{minimum>>16}.{minimum>>8&255}','adhoc_valid':signed,'verified_pages':pages})
    assert {x['cpu'] for x in slices}=={'0x1000007','0x100000c'}
    return slices

for platform in ['linux-x64','windows-x64','macos-universal']:
    prefix=f'mm2-cht-{platform}-local-full-{version}'
    if platform=='linux-x64':
        artifact=release/'full-local'/(prefix+'.AppImage')
        resource=linux/'squashfs-root/usr'
        read=lambda name:(resource/name).read_bytes()
        names={p.relative_to(resource).as_posix() for p in resource.rglob('*') if p.is_file()}
        assert (linux/'squashfs-root/AppRun').stat().st_mode&0o111
    else:
        artifact=release/'full-local'/(prefix+'.zip')
        z=zipfile.ZipFile(artifact)
        assert z.testzip() is None
        assert all(e.flag_bits&0x800 for e in z.infolist())
        base=prefix+'/'
        if platform=='macos-universal':
            base+='MM2-CHT.app/Contents/Resources/'
            info=plistlib.loads(z.read(prefix+'/MM2-CHT.app/Contents/Info.plist'))
            assert info['MM2ReleaseVersion']==version and info['LSMinimumSystemVersion']=='12.0'
            launcher=z.getinfo(prefix+'/MM2-CHT.app/Contents/MacOS/mm2-cht')
            assert launcher.external_attr>>16&0o111
        read=lambda name:z.read(base+name)
        names={n[len(base):] for n in z.namelist() if n.startswith(base) and not n.endswith('/')}
    assert read('VERSION').decode().strip()==version
    manifest=json.loads(read('ASSET-MANIFEST.json'))
    for name,expected in manifest['files'].items():
        assert hashlib.sha256(read(name)).hexdigest()==expected,(platform,name)
    assert 'save/state.json' not in names
    assert not any('/masters/' in name or '/cache/' in name for name in names)
    hd=json.loads(read('workplace/hd-amiga-r1/hd.json'))
    assert hd['expected']==hd['generated']==len(hd['entries'])==930
    for e in hd['entries']:
        assert hashlib.sha256(read('workplace/hd-amiga-r1/'+e['art'])).hexdigest()==e['art_sha256']
    music=json.loads(read('music/manifest.json'))
    assert len(music['tracks'])==16 and all(p.endswith('.ogg') for p in music['tracks'].values())
    assert all(read('music/'+p).startswith(b'OggS') for p in music['tracks'].values())
    assert len([n for n in names if n.startswith('workplace/msx/') and n.endswith('.dsk')])==2
    item={'file':artifact.name,'bytes':artifact.stat().st_size,'sha256':hashlib.sha256(artifact.read_bytes()).hexdigest(),'resource_files':len(names),'manifest_hashes_valid':True,'hd_entries':930,'music_roles':16}
    if platform=='macos-universal':
        item['mach_o']={name:macho(read('bin/'+name)) for name in ['mm2','mm2data']}
    if platform=='windows-x64':
        assert read('bin/mm2.exe')[:2]==b'MZ'
        batch=read('run.bat');assert b'\r\n' in batch and b'\n' not in batch.replace(b'\r\n',b'')
        assert b'%LOCALAPPDATA%' in batch
    shared.append(hashlib.sha256(read('SOURCE-MANIFEST.json')).hexdigest())
    item['source_manifest_sha256']=shared[-1]
    result['platforms'][platform]=item
assert len(set(shared))==1,'platforms used different source inventories'
(release/'smoke/package-verification.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result,indent=2))
