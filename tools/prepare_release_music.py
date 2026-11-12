#!/usr/bin/env python3
"""由已擷取的原版 WAV 產生正式 OGG 音樂包與可追溯收據。"""
import hashlib
import array
import math
import json
from pathlib import Path
import subprocess
import sys

source, target = map(Path, sys.argv[1:3])
target.mkdir(parents=True, exist_ok=True)
manifest = json.loads((source/'manifest.json').read_text())
receipts = []
def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def probe(p):
    return json.loads(subprocess.check_output(['ffprobe','-v','error','-show_streams','-show_format','-of','json',str(p)]))
for filename in sorted(set(manifest['tracks'].values())):
    inp, out = source/filename, target/(Path(filename).stem+'.ogg')
    a=probe(inp);duration=float(a['format']['duration'])
    effects=f'volume=-6dB,afade=t=in:d=0.02,afade=t=out:st={duration-.02:.6f}:d=0.02'
    command = ['ffmpeg','-nostdin','-v','error','-y','-i',str(inp),'-map','0:a:0','-af',effects,'-c:a','libvorbis','-q:a','6','-threads','2',str(out)]
    subprocess.run(command,check=True)
    b = probe(out)
    assert b['streams'][0]['codec_name']=='vorbis'
    assert abs(float(a['format']['duration'])-float(b['format']['duration']))<.1
    pcm=array.array('h',subprocess.check_output(['ffmpeg','-v','error','-i',str(out),'-f','s16le','-acodec','pcm_s16le','-']))
    peak=max(map(abs,pcm));rms=math.sqrt(sum(x*x for x in pcm)/len(pcm))
    seam=max(abs(pcm[-2]-pcm[0]),abs(pcm[-1]-pcm[1]))
    assert peak<32760 and rms>100 and seam<1024,(out,peak,rms,seam)
    receipts.append({'input':filename,'input_sha256':sha(inp),'output':out.name,'output_sha256':sha(out),'duration':b['format']['duration'],'sample_rate':b['streams'][0]['sample_rate'],'channels':b['streams'][0]['channels'],'pcm_peak':peak,'pcm_rms':round(rms,2),'loop_boundary_delta':seam,'command':command})
manifest['tracks']={role:Path(name).stem+'.ogg' for role,name in manifest['tracks'].items()}
(target/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
if (source/'manifest.txt').is_file():
    (target/'SOURCE.txt').write_bytes((source/'manifest.txt').read_bytes())
(target/'ENCODING.json').write_text(json.dumps({'version':Path('/src/VERSION').read_text().strip(),'format':'OGG Vorbis quality 6','gain_db':-6,'edge_fade_seconds':.02,'note':'Original emulated capture may already contain clipped samples; gain prevents additional output clipping. Short fades soften the loop seam; musical bar alignment and listening are not certified. No source restoration is claimed.','ffmpeg':subprocess.check_output(['ffmpeg','-version']).decode().splitlines()[0],'files':receipts},indent=2)+'\n')
print(f'Encoded {len(receipts)} OGG tracks')
