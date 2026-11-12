#!/usr/bin/env bash
# Docker only. FFmpeg threads stay bounded; no zoompan frame multiplication.
set -euo pipefail
work=${1:?release working directory}
output=${2:?final mp4}
captures=${3:-capture-r2}
cd "$work"
mkdir -p edited
python3 - "$work" <<'PY'
from pathlib import Path
import sys
p=Path(sys.argv[1]);names=['02-preedit.png','04-ime-confirm.png','05-character-created.png','06-reloaded.png']
available=list((p/'input-gui').glob('*.png'))
print('Input captures:',[x.name for x in available])
names=['02-preedit.png','03-preedit-second.png','04-ime-confirm.png','04-ime-confirm.png']
paths=[]
for name in names:
 f=p/'input-gui'/name
 if f.exists():paths.append(f)
if len(paths)<4:
 paths=[p/'input-gui'/n for n in ['02-preedit.png','02b-ime-cancel.png','03-preedit-second.png','04-ime-confirm.png']]
assert all(x.is_file() for x in paths)
with (p/'input.ffconcat').open('w') as f:
 f.write('ffconcat version 1.0\n')
 for x in paths:f.write(f"file '{x}'\nduration 2\n")
 f.write(f"file '{paths[-1]}'\n")
PY
ffmpeg -nostdin -v error -y -safe 0 -f concat -i input.ffconcat -t 8 -r 30 -an -c:v libx264 -preset veryfast -crf 16 -threads 2 input.mkv
inputs=("$captures/00-intro.mkv" "$captures/01-explore.mkv" "$captures/02-toolbar.mkv" "$captures/03-facility.mkv" "$captures/04-map.mkv" input.mkv battle.mkv "$captures/00-intro.mkv")
durations=(10 10 10 10 8 8 12 4)
for index in $(seq 0 7); do
  num=$(printf '%02d' "$index")
  if [[ -n "${MM2_RENDER_SCENE:-}" && "$MM2_RENDER_SCENE" != "$index" ]]; then test -s "edited/$num.mkv"; continue; fi
  ffmpeg -nostdin -v error -y -i "${inputs[$index]}" -loop 1 -i "frames/frame-$num.png" \
    -filter_complex '[0:v]setpts=PTS-STARTPTS,scale=1374:916:flags=lanczos,pad=1920:1080:44:110:color=0x0a121f[game];[game][1:v]overlay=0:0:shortest=1,fps=30,tpad=stop_mode=clone:stop_duration=2,setsar=1,format=yuv420p[v]' \
    -map '[v]' -an -t "${durations[$index]}" -r 30 -fps_mode cfr -c:v libx264 -crf 18 -preset veryfast -threads 2 "edited/$num.mkv"
done
for index in $(seq 0 7); do printf "file '%02d.mkv'\n" "$index"; done > edited/concat.txt
ffmpeg -nostdin -v error -y -f concat -safe 0 -i edited/concat.txt -c copy silent.mkv
music=/src/workplace/genesis/music
version=$(cat /src/VERSION)
ffmpeg -nostdin -v error -y -i silent.mkv \
  -i "$music/md-0B8AE0.wav" -i "$music/md-0B48D4.wav" -i "$music/md-0B2290.wav" -i "$music/md-0B8224.wav" \
  -filter_complex '[1:a]atrim=0:20,asetpts=PTS-STARTPTS,volume=-6dB[a0];[2:a]atrim=0:29,asetpts=PTS-STARTPTS,volume=-6dB[a1];[3:a]atrim=0:9,asetpts=PTS-STARTPTS,volume=-6dB[a2];[4:a]atrim=0:17,asetpts=PTS-STARTPTS,volume=-6dB[a3];[a0][a1]acrossfade=d=1:c1=tri:c2=tri[b0];[b0][a2]acrossfade=d=1:c1=tri:c2=tri[b1];[b1][a3]acrossfade=d=1:c1=tri:c2=tri,loudnorm=I=-18:TP=-1.5:LRA=11,aresample=48000,afade=t=in:st=0:d=1,afade=t=out:st=70:d=2[a]' \
  -map 0:v -map '[a]' -t 72 -c:v copy -c:a aac -b:a 192k -ar 48000 -movflags +faststart \
  -metadata title="魔法門 II HD｜$version" -metadata comment='Local-only original-derived artwork and Mega Drive audio. Current GUI capture plus a controlled battle fixture.' "$output"
