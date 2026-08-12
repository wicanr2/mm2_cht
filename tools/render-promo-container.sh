#!/usr/bin/env bash
set -euo pipefail

shots=(
  00-chinese
  01-first-person
  01c-first-person-amiga
  01f-first-person-msx
  01e-first-person-pack
  10-create
  03-items
  04-shop
  02-cast
  07-combat
  08-protection
  06-map
  05-reference
  12-worldmap
  00-chinese
)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
segments="$work/segments.txt"
last=$((${#shots[@]} - 1))
for i in "${!shots[@]}"; do
  f="$PWD/shots/${shots[$i]}.png"
  [[ -s "$f" ]] || { echo "缺少推廣片畫面：$f" >&2; exit 1; }
  vf='scale=1800:1080:flags=neighbor,pad=1920:1080:60:0:black,setsar=1,format=yuv420p'
  if ((i == 0)); then
    vf+=',fade=t=in:st=0:d=1'
  fi
  if ((i == last)); then
    vf+=',fade=t=out:st=2:d=2'
  fi
  seg=$(printf '%s/segment-%02d.mp4' "$work" "$i")
  ffmpeg -hide_banner -loglevel error -y -threads 1 -filter_threads 1 \
    -loop 1 -framerate 30 -i "$f" -t 4 -vf "$vf" -an \
    -c:v libx264 -preset veryfast -tune stillimage -crf 19 -pix_fmt yuv420p "$seg"
  printf "file '%s'\n" "$seg" >> "$segments"
done

ffmpeg -hide_banner -loglevel error -y -f concat -safe 0 -i "$segments" \
  -c copy "$work/video.mp4"

# 〈異界之門：五步〉：完全由 FFmpeg 振盪器生成的新作。
# 低音以 D/A 五度維持世界感；上聲部每四秒走 D–F–A–G–E 五音動機。
audio="aevalsrc=exprs='0.055*sin(2*PI*73.416*t)+0.035*sin(2*PI*110*t)+0.025*sin(2*PI*(if(lt(mod(t\,4)\,0.8)\,293.665\,if(lt(mod(t\,4)\,1.6)\,349.228\,if(lt(mod(t\,4)\,2.4)\,440\,if(lt(mod(t\,4)\,3.2)\,391.995\,329.628)))))*t)':s=48000:d=60"

ffmpeg -hide_banner -loglevel warning -y -threads 1 \
  -i "$work/video.mp4" -f lavfi -i "$audio" \
  -af 'afade=t=in:st=0:d=2,afade=t=out:st=57:d=3,volume=0.8' -t 60 \
  -c:v copy \
  -c:a aac -b:a 160k -movflags +faststart mm2-remake-trailer.mp4

ffprobe -v error -show_entries format=duration,size \
  -show_entries stream=index,codec_name,width,height,sample_rate \
  -of default=noprint_wrappers=1 mm2-remake-trailer.mp4
