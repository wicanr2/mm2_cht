# Might and Magic II (New World Computing, 1988) 逆向工程參考環境
# DOSBox 0.74-3（Debian bookworm 套件版本）+ 螢幕截圖 / 自動化輸入所需工具
#
# 用途：作為「原版遊戲實際行為」的 reference oracle，不用於長期互動遊玩。
# 全程 headless（無 X display），靠 Xvfb 起虛擬 framebuffer，
# 用 xdotool 送鍵、xwd + ImageMagick 截圖。
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        dosbox \
        xvfb \
        x11-apps \
        imagemagick \
        xdotool \
        procps \
        python3 \
    && rm -rf /var/lib/apt/lists/*

# ImageMagick 6 在 Debian 預設的 policy.xml 會擋掉部份 coder，
# 這裡用不到會被擋的 PDF/EPS 之類格式，PNG/XWD 不受影響，不需另外改 policy。

WORKDIR /work

# 實際遊戲資料由 tools/dosbox_run.sh 用 bind mount 掛進來（/game），
# 截圖輸出也用 bind mount（/shots）。image 本身不含任何遊戲資產
# （版權因素 + 保持 image 通用可重建）。

COPY entrypoint.sh /usr/local/bin/dosbox-entrypoint.sh
RUN chmod +x /usr/local/bin/dosbox-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/dosbox-entrypoint.sh"]
