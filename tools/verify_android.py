#!/usr/bin/env python3
"""容器內核對 Android 本機包、原生對齊及內嵌素材。"""
import argparse
import hashlib
import io
import json
from pathlib import Path
import shutil
import struct
import subprocess
import zipfile


def digest(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('apk', type=Path)
    parser.add_argument('--receipt', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--deliver', action='store_true')
    parser.add_argument('--evidence-dir', type=Path)
    args = parser.parse_args()
    assert Path('/.dockerenv').exists()
    receipt = json.loads(args.receipt.read_text())
    assert digest(args.apk.read_bytes()) == receipt['sha256']
    source = json.loads((args.receipt.parent/'aar-source.json').read_text())
    assert source['fingerprint'] == receipt['source_fingerprint']
    for name, expected in source['files'].items():
        assert digest((Path('/src')/name).read_bytes()) == expected, 'program source changed: '+name
    for name, expected in receipt['native_inputs'].items():
        assert digest((Path('/src')/name).read_bytes()) == expected, 'native source changed: '+name
    tools = Path('/opt/android-sdk/build-tools/35.0.0')
    subprocess.run([tools/'apksigner', 'verify', '--verbose', args.apk], check=True)
    subprocess.run([tools/'zipalign', '-c', '-P', '16', '4', args.apk], check=True)
    libraries = {}
    with zipfile.ZipFile(args.apk) as apk:
        assert apk.testzip() is None
        assert apk.read('assets/VERSION').decode().strip() == receipt['version']
        assert sorted(n for n in apk.namelist() if n.startswith('lib/')) == receipt['abis']
        for name in receipt['abis']:
            elf = apk.read(name)
            assert elf[:4] == b'\x7fELF' and elf[5] == 1
            wide = elf[4] == 2
            offset = struct.unpack_from('<Q' if wide else '<I', elf, 32 if wide else 28)[0]
            entry_size, count = struct.unpack_from('<HH', elf, 54 if wide else 42)
            loads = []
            for index in range(count):
                entry = struct.unpack_from('<IIQQQQQQ' if wide else '<IIIIIIII', elf, offset+index*entry_size)
                if entry[0] != 1:
                    continue
                file_offset, address, alignment = (entry[2], entry[3], entry[7]) if wide else (entry[1], entry[2], entry[7])
                assert alignment >= 16384 and (address-file_offset) % 16384 == 0, (name, entry)
                loads.append(alignment)
            assert loads
            libraries[name] = {'load_alignments': loads, 'sha256': digest(elf)}
        runtime = apk.read('assets/runtime.zip')
        assert digest(runtime) == receipt['runtime_sha256'] == apk.read('assets/runtime.sha256').decode().strip()
    with zipfile.ZipFile(io.BytesIO(runtime)) as assets:
        assert assets.testzip() is None
        manifest = json.loads(assets.read('ANDROID-ASSETS.json'))
        assert manifest['version'] == receipt['version']
        assert set(assets.namelist()) == set(manifest['files']) | {'ANDROID-ASSETS.json'}
        for name, expected in manifest['files'].items():
            assert digest(assets.read(name)) == expected, name
            assert not name.startswith(('save/', 'cache/', 'workplace/android/')), name
        for name in ['workplace/orig/MM2/MAP.DAT', 'workplace/orig/MM2/MM2.EXE',
                     'workplace/hd-amiga-r1/frame.png', 'assets/font/input-sc24.bin',
                     'assets/font/input-tc24.bin', 'translations/interface.tsv',
                     'music/manifest.json', 'THIRD-PARTY/Go-LICENSE.txt', 'LICENSE']:
            assert name in manifest['files'], name
    result = {'version': receipt['version'], 'apk_sha256': receipt['sha256'],
              'libraries': libraries, 'runtime_files_verified': len(manifest['files']),
              'signature': 'passed', 'zip_alignment_16kb': 'passed',
              'elf_alignment_16kb': 'passed', 'runtime_hashes': 'passed',
              'rights': 'local only; original resources and derivative artwork',
              'scope': 'package verification; does not replace device playtesting'}
    args.output.write_text(json.dumps(result, indent=2)+'\n')
    if args.deliver:
        assert args.evidence_dir and args.evidence_dir.is_dir()
        for name in ['initial-smoke.json', 'player-smoke.json', 'items-smoke.json']:
            evidence = json.loads((args.evidence_dir/name).read_text())
            assert evidence['apk_sha256'] == receipt['sha256'], 'playtest belongs to another APK'
            if name != 'initial-smoke.json':
                assert evidence['result'] == 'passed'
        destination = Path('/src/dist-all')/receipt['version']
        assert not destination.exists(), 'delivery already exists; do not overwrite'
        assert destination.parent.is_dir() and destination.parent.stat().st_uid == args.apk.stat().st_uid
        full = destination/'full-local'
        smoke = destination/'smoke'
        full.mkdir(parents=True)
        smoke.mkdir()
        shutil.copy2(args.apk, full/('mm2-cht-android-local-full-'+receipt['version']+'.apk'))
        shutil.copytree(args.evidence_dir, smoke/'android', dirs_exist_ok=True)
        for name in ['apk-check.json', 'apk-badging.txt', 'package-check.json', 'aar-source.json', 'runtime-source.json',
                     'go-regression-final.log', 'go-vet-final.log', 'mobile-combat.log', 'mobile-final.log', 'release-check.log',
                     'build-environment.json']:
            shutil.copy2(args.receipt.parent/name, smoke/name)
        files = {p.relative_to(destination).as_posix(): {'bytes': p.stat().st_size, 'sha256': digest(p.read_bytes())}
                 for p in sorted(destination.rglob('*')) if p.is_file()}
        manifest = {'version': receipt['version'], 'rights': 'local only',
                    'signing': receipt['signing'], 'files': files,
                    'limits': 'API23 software emulator; physical Android phones, API35 and third-party pinyin IMEs not accepted'}
        (destination/'SHA256SUMS.json').write_text(json.dumps(manifest, indent=2)+'\n')
        print('Delivery:', destination)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
