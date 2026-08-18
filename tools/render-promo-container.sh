#!/usr/bin/env bash
set -euo pipefail

# 三段式推廣片：素材拍攝 → 章節卡片 → 固定畫面剪輯。所有原版衍生檔仍只在
# workplace/promo；這個容器只讀取玩家自備 MM2.EXE 轉出的本機 WAV。
font='/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc'
font_bold='/usr/share/fonts/opentype/noto/NotoSerifCJK-Bold.ttc'
mkdir -p cards

card() {
  local out="$1" title="$2" body="$3" accent="$4"
  convert -size 1920x1080 xc:'#090606' \
    -fill "$accent" -draw 'rectangle 55,55 1865,1025' \
    -fill '#5B0403' -draw 'rectangle 80,80 1840,1000' \
    -fill '#090606' -draw 'rectangle 96,96 1824,984' \
    -font "$font_bold" -pointsize 76 -fill "$accent" -gravity NorthWest \
    -annotate +150+190 "$title" \
    -font "$font" -pointsize 42 -fill '#AFDDEF' -gravity NorthWest \
    -annotate +155+335 "$body" \
    -font "$font" -pointsize 30 -fill '#A75505' -gravity SouthEast \
    -annotate +145+120 'MIGHT & MAGIC II  •  繁體中文 remake' "$out"
}

card cards/00-title.png '魔法門 II' '一場從 DOS 時代重新走進繁中的冒險' '#970404'
card cards/01-hook.png '老世界，新入口' '原版資料作為行為 oracle；Go／Ebiten 引擎重新接起玩家路徑' '#2F46BF'
card cards/02-archive.png '先看證據，再做 remake' '對話、地圖、物品、戰鬥與存檔，沿著正常 UI 逐段驗證' '#A75505'
card cards/03-themes.png '同一套冒險，多種時代外觀' 'DOS／Amiga／MSX 素材組可切換；現代 Theme 保持可替換' '#505AF3'
card cards/08-rules.png '規則進入畫面' '施法目標、物品數字提示與戰術回饋，直接接上玩家操作' '#970404'
card cards/11-monsters.png '怪物動畫：正常 UI 已接上' '433 張原版影格、181 段序列；目前保守播放第一個合法序列（強推論）' '#A75505'
card cards/15-close.png '把原版帶回來' '自備合法原版資料；公開 repo 不含原版執行檔、資料、美術或音樂' '#970404'

items=(
  cards/00-title.png cards/01-hook.png shots/00-chinese.png
  cards/02-archive.png shots/01-first-person.png shots/01c-first-person-amiga.png
  shots/01f-first-person-msx.png cards/03-themes.png shots/01e-first-person-pack.png
  shots/10-create.png shots/03-items.png shots/04-shop.png
  cards/08-rules.png shots/02-cast.png shots/07-combat.png
  cards/11-monsters.png combat-ui-animation shots/08-protection.png shots/06-map.png
  shots/05-reference.png shots/12-worldmap.png cards/15-close.png
)
durations=(
  4 4 3
  4 4 4 4 2 2
  4 4 4 4 4 4
  3 3 2 2 2 2 3
)

if ((${#items[@]} != ${#durations[@]})); then
  echo "推廣片素材與片長表數量不一致" >&2
  exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
segments="$work/segments.txt"
last=$((${#items[@]} - 1))
for i in "${!items[@]}"; do
  f="$PWD/${items[$i]}"
  duration=${durations[$i]}
  if [[ ${items[$i]} == combat-ui-animation ]]; then
    frames=(
      "$PWD/shots/07a-combat-anim-00.png"
      "$PWD/shots/07a-combat-anim-15.png"
    )
    for f in "${frames[@]}"; do
      [[ -s "$f" ]] || { echo "缺少怪物研究影格：$f" >&2; exit 1; }
    done
    frame_list="$work/monster-frames.txt"
    : > "$frame_list"
    # 這兩張都由正常 Session.Draw 路徑產生，中間經過十五個 Tick；來回播放
    # 只證明 remake 已接上動畫，不主張第一段就是原版戰鬥待機用途。
    frame_no=0
    for repeat in {1..5}; do
      for f in "${frames[@]}"; do
        frame_seg=$(printf '%s/monster-%02d.mp4' "$work" "$frame_no")
        ffmpeg -hide_banner -loglevel error -y -threads 1 -filter_threads 1 \
          -loop 1 -framerate 30 -i "$f" -t 0.3 \
          -vf 'scale=1800:1012:flags=neighbor,pad=1920:1080:60:34:#A75505,setsar=1,format=yuv420p' \
          -an -c:v libx264 -preset veryfast -tune stillimage -crf 19 -pix_fmt yuv420p "$frame_seg"
        printf "file '%s'\n" "$frame_seg" >> "$frame_list"
        frame_no=$((frame_no + 1))
      done
    done
    seg=$(printf '%s/segment-%02d.mp4' "$work" "$i")
    ffmpeg -hide_banner -loglevel error -y -f concat -safe 0 -i "$frame_list" -c copy "$seg"
    printf "file '%s'\n" "$seg" >> "$segments"
    continue
  fi
  [[ -s "$f" ]] || { echo "缺少推廣片素材：$f" >&2; exit 1; }
  # 奇數畫面以 EGA 藍框、偶數畫面以血紅框，保留像素邊緣，不使用縮放動畫。
  border='#970404'
  (( i % 2 )) && border='#2F46BF'
  vf="scale=1800:1012:flags=neighbor,pad=1920:1080:60:34:${border},setsar=1,format=yuv420p"
  if ((i == 0)); then vf+=',fade=t=in:st=0:d=1'; fi
  if ((i == last)); then vf+=",fade=t=out:st=$((duration - 2)):d=2"; fi
  seg=$(printf '%s/segment-%02d.mp4' "$work" "$i")
  ffmpeg -hide_banner -loglevel error -y -threads 1 -filter_threads 1 \
    -loop 1 -framerate 30 -i "$f" -t "$duration" -vf "$vf" -an \
    -c:v libx264 -preset veryfast -tune stillimage -crf 19 -pix_fmt yuv420p "$seg"
  printf "file '%s'\n" "$seg" >> "$segments"
done

ffmpeg -hide_banner -loglevel error -y -f concat -safe 0 -i "$segments" -c copy "$work/video.mp4"

audio=/out/music/mm2-original-pc-speaker.wav
[[ -s "$audio" ]] || { echo "缺少本機原版音源：$audio（請先執行 mm2music）" >&2; exit 1; }

# 兩個配樂變體共用同一段畫面，只換音軌：
#   mm2-remake-trailer.mp4            DOS 的 PC 喇叭方波，由 MM2.EXE 的音高／時值表離線轉譯
#   mm2-remake-trailer-megadrive.mp4  Mega Drive 版的原始配樂（本機有音樂包才做）
# 兩個都是原版衍生內容，只留在 workplace/promo，不對外散布。
mux() {
  local src="$1" out="$2"
  ffmpeg -hide_banner -loglevel warning -y -threads 1 -stream_loop -1 \
    -i "$src" -i "$work/video.mp4" \
    -map 1:v:0 -map 0:a:0 -af 'afade=t=in:st=0:d=2,afade=t=out:st=68:d=4,volume=0.8' -t 72 \
    -c:v copy -c:a aac -b:a 160k -movflags +faststart "$out"
  ffprobe -v error -show_entries format=duration,size \
    -show_entries stream=index,codec_name,width,height,sample_rate \
    -of default=noprint_wrappers=1 "$out"
}

mux "$audio" mm2-remake-trailer.mp4
md=/out/music/mm2-megadrive-medley.wav
if [[ -s "$md" ]]; then
  mux "$md" mm2-remake-trailer-megadrive.mp4
fi
