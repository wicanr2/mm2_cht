# BlastEm 0.6.3-pre：Mega Drive 音樂擷取（VGM）用的固定版本環境
#
# 為什麼要固定版本：VGM 記錄的是暫存器寫入的時序，換一版模擬器就可能換掉音色與
# 時序，先前錄的東西無法與新錄的比較。這裡把 nightly 的 commit 釘死在 image 裡。
#
# 為什麼要 Xvfb + xdotool：BlastEm 的 VGM 記錄只有熱鍵 `m`，沒有命令列旗標、
# 沒有 headless 模式（vgmrips wiki 已確認）。所以只能起虛擬 framebuffer 再送鍵，
# 與本專案跑 DOSBox 當 oracle 完全同一套做法。
#
# 0.6.2 正式版沒有 VGM 記錄功能，按 `m` 完全沒反應。必須 0.6.3 以上的 nightly。
#
# image 本身不含任何遊戲資產：ROM 由呼叫端唯讀 bind mount 到 /rom，
# 輸出到 /out。
# --- 第一階段：編 libvgm 的 vgm2wav（VGM 是暫存器寫入的時間序列，不是音訊，
#     轉檔＝重跑一次晶片模擬，所以這一步換版本會換音色，要跟著記進中介資料）---
FROM debian:bookworm-slim AS vgmbuild

# 釘死 commit：換版本會換晶片模擬，也就換掉音色，先前錄的與新錄的不可混用。
ARG LIBVGM_REF=61fc6725644886abc3168e240e4e51588d74bdf7

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates git cmake build-essential zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

RUN git clone https://github.com/ValleyBell/libvgm.git /src/libvgm \
    && cd /src/libvgm \
    && git checkout "${LIBVGM_REF}" \
    && git rev-parse HEAD > /src/LIBVGM_COMMIT \
    && cmake -S . -B build \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_TESTS=OFF \
        -DBUILD_PLAYER=OFF \
        -DBUILD_VGM2WAV=ON \
        -DUSE_SANITIZERS=OFF \
    && cmake --build build -j"$(nproc)" --target vgm2wav

FROM debian:bookworm-slim

# 釘死的 nightly。要換版本時連同 docs/research/md-music-driver.md 一起改，
# 並重錄全部曲目 —— 不同版本的輸出不可混用。
ARG BLASTEM_BUILD=blastem64-0.6.3-pre-732f5689d438

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        xvfb \
        xdotool \
        x11-apps \
        imagemagick \
        procps \
        libsdl2-2.0-0 \
        libgl1 \
        libglu1-mesa \
        libgl1-mesa-dri \
        fontconfig \
        fonts-dejavu-core \
        python3-minimal \
    && rm -rf /var/lib/apt/lists/*
# BlastEm 用 fc-match 找預設字型畫自己的選單；找不到會印
# 「Failed to find default font path」然後在啟動後隨即結束 —— 看起來像 ROM
# 載入失敗，其實是缺字型。fontconfig 與字型兩個都要，只裝其中一個沒有用。

RUN curl -fsSL "https://www.retrodev.com/blastem/nightlies/${BLASTEM_BUILD}.tar.gz" \
        -o /tmp/blastem.tar.gz \
    && mkdir -p /opt/blastem \
    && tar -xzf /tmp/blastem.tar.gz -C /opt/blastem --strip-components=1 \
    && rm /tmp/blastem.tar.gz \
    && echo "${BLASTEM_BUILD}" > /opt/blastem/BUILD_ID

# 容器一律以呼叫者的 UID/GID 執行（不是 root），所以暫存目錄要先開成任何人可寫。
# 不做這一步的症狀是 `cp: cannot create regular file '/work/rom.md': Permission denied`，
# 看起來像 ROM 掛載錯了，其實是 image 內的目錄權限。
# /tmp/.X11-unix 同理：非 root 的 Xvfb 建不出來，會噴 euid != 0。
COPY --from=vgmbuild /src/libvgm/build/bin/vgm2wav /usr/local/bin/vgm2wav
COPY --from=vgmbuild /src/LIBVGM_COMMIT /opt/blastem/LIBVGM_COMMIT

RUN mkdir -p /work /tmp/.X11-unix && chmod 1777 /work /tmp/.X11-unix

WORKDIR /work

COPY entrypoint.sh /usr/local/bin/blastem-entrypoint.sh
COPY music-entrypoint.sh /usr/local/bin/blastem-music-entrypoint.sh
COPY vgm2pcmwav.py /usr/local/bin/vgm2pcmwav
COPY rsp.py /usr/local/bin/rsp.py
COPY music_dump.py /usr/local/bin/md-music-dump
COPY md_trace.py /usr/local/bin/md-trace
RUN chmod +x /usr/local/bin/blastem-entrypoint.sh \
             /usr/local/bin/blastem-music-entrypoint.sh \
             /usr/local/bin/vgm2pcmwav \
             /usr/local/bin/md-music-dump \
             /usr/local/bin/md-trace

ENTRYPOINT ["/usr/local/bin/blastem-entrypoint.sh"]
