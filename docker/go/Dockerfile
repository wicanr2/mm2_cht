# Go + Ebiten 建置／測試環境
#
# 硬規則：本專案一律用 docker 編譯，不污染主機 Go 環境。
# Ebiten 需要 X11 / OpenGL 的標頭檔才能編譯（即使跑 headless 測試也要，
# 因為 cmd/demonwinter 會 import ebiten）。
FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
        libgl1-mesa-dev \
        libxcursor-dev \
        libxi-dev \
        libxinerama-dev \
        libxrandr-dev \
        libxxf86vm-dev \
        libasound2-dev \
        pkg-config \
        # headless 跑 Ebiten 並截圖用：本專案硬規則是視覺產物一律 dump PNG
        # 肉眼比對，不接受「編譯過了」當驗收。
        xvfb \
        x11-utils \
        xdotool \
        imagemagick \
        # 宣傳片的實機片段用 x11grab 錄製；仍封在 Docker，不污染主機。
        ffmpeg \
        mesa-utils \
    && rm -rf /var/lib/apt/lists/*

# module cache 掛在 volume 上，避免每次 rebuild 重抓
ENV GOPATH=/go
ENV GOFLAGS=-buildvcs=false
ENV CGO_ENABLED=1

WORKDIR /src
