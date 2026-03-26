#!/bin/bash
# build/build_ffmpeg_minimal.sh
# Docker で Windows 向け ffmpeg/ffprobe を最小構成でクロスコンパイルする
#
# 使い方:
#   chmod +x build/build_ffmpeg_minimal.sh
#   ./build/build_ffmpeg_minimal.sh
#
# 出力: dist/bin/ffmpeg.exe, dist/bin/ffprobe.exe

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="$PROJECT_ROOT/dist/bin"

IMAGE_NAME="h265conv-ffmpeg-builder"

mkdir -p "$OUTPUT_DIR"

echo "=== Building minimal ffmpeg for Windows x64 ==="

docker build -t "$IMAGE_NAME" -f - "$PROJECT_ROOT/build" <<'DOCKERFILE'
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    mingw-w64 \
    pkg-config \
    nasm \
    yasm \
    git \
    curl \
    ca-certificates \
    cmake \
    upx-ucl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# ── 1. nv-codec-headers (NVENC API headers, no runtime dependency) ──
RUN git clone --depth 1 --branch n12.2.72.0 \
      https://github.com/FFmpeg/nv-codec-headers.git /build/nv-codec-headers && \
    cd /build/nv-codec-headers && \
    make PREFIX=/build/prefix install

# ── 2. x265 (static, Windows cross-compile) ──
RUN git clone --depth 1 --branch 4.1 \
      https://bitbucket.org/multicoreware/x265_git.git /build/x265

RUN mkdir -p /build/x265/build-win && cd /build/x265/build-win && \
    cmake ../source \
      -DCMAKE_SYSTEM_NAME=Windows \
      -DCMAKE_C_COMPILER=x86_64-w64-mingw32-gcc \
      -DCMAKE_CXX_COMPILER=x86_64-w64-mingw32-g++ \
      -DCMAKE_RC_COMPILER=x86_64-w64-mingw32-windres \
      -DCMAKE_INSTALL_PREFIX=/build/prefix \
      -DENABLE_SHARED=OFF \
      -DENABLE_CLI=OFF \
      -DHIGH_BIT_DEPTH=OFF \
      -DENABLE_HDR10_PLUS=OFF \
      -DENABLE_ASSEMBLY=OFF \
    && make -j$(nproc) && make install

# Fix x265 pkg-config (add missing C++ and thread libs for static link)
RUN sed -i 's|Libs: .*|Libs: -L${libdir} -lx265 -lstdc++ -lpthread|' \
      /build/prefix/lib/pkgconfig/x265.pc && \
    cat /build/prefix/lib/pkgconfig/x265.pc

# ── 3. ffmpeg source ──
RUN curl -L "https://ffmpeg.org/releases/ffmpeg-7.1.tar.xz" | tar xJ

# ── 4. Create cross pkg-config symlink ──
# ffmpeg's configure prepends --cross-prefix to "pkg-config", calling
# "x86_64-w64-mingw32-pkg-config". We symlink it to the real pkg-config.
RUN ln -sf /usr/bin/pkg-config /usr/local/bin/x86_64-w64-mingw32-pkg-config

# ── 5. Configure & build ffmpeg ──
ENV PKG_CONFIG_LIBDIR=/build/prefix/lib/pkgconfig
ENV PKG_CONFIG_PATH=/build/prefix/lib/pkgconfig
RUN cd /build/ffmpeg-7.1 && \
    ./configure \
      --arch=x86_64 \
      --target-os=mingw32 \
      --cross-prefix=x86_64-w64-mingw32- \
      --prefix=/build/out \
      --pkg-config-flags="--static" \
      --extra-libs="-lstdc++ -lpthread" \
      \
      --enable-gpl \
      --enable-version3 \
      --enable-static \
      --disable-shared \
      --disable-debug \
      --disable-doc \
      --disable-htmlpages \
      --disable-manpages \
      --disable-podpages \
      --disable-txtpages \
      \
      --enable-libx265 \
      --enable-nvenc \
      --enable-ffnvcodec \
      \
      --disable-programs \
      --enable-ffmpeg \
      --enable-ffprobe \
      \
      --disable-avdevice \
      --disable-postproc \
      --enable-avfilter \
      --enable-swresample \
      --enable-swscale \
      \
      --disable-network \
      --enable-protocol=file \
      --enable-protocol=pipe \
      \
      --disable-encoders \
      --enable-encoder=hevc_nvenc \
      --enable-encoder=libx265 \
      --enable-encoder=aac \
      \
      --disable-decoders \
      --enable-decoder=h264 \
      --enable-decoder=hevc \
      --enable-decoder=vp8 \
      --enable-decoder=vp9 \
      --enable-decoder=av1 \
      --enable-decoder=mpeg4 \
      --enable-decoder=mpeg2video \
      --enable-decoder=wmv3 \
      --enable-decoder=vc1 \
      --enable-decoder=flv \
      --enable-decoder=aac \
      --enable-decoder=mp3 \
      --enable-decoder=opus \
      --enable-decoder=vorbis \
      --enable-decoder=flac \
      --enable-decoder=ac3 \
      --enable-decoder=eac3 \
      --enable-decoder=pcm_s16le \
      --enable-decoder=pcm_s24le \
      --enable-decoder=pcm_f32le \
      --enable-decoder=mjpeg \
      --enable-decoder=png \
      --enable-decoder=rawvideo \
      \
      --disable-demuxers \
      --enable-demuxer=mov \
      --enable-demuxer=matroska \
      --enable-demuxer=avi \
      --enable-demuxer=asf \
      --enable-demuxer=flv \
      --enable-demuxer=mpegts \
      --enable-demuxer=mpegps \
      --enable-demuxer=wav \
      --enable-demuxer=ogg \
      --enable-demuxer=webm_dash_manifest \
      --enable-demuxer=concat \
      \
      --disable-muxers \
      --enable-muxer=mp4 \
      --enable-muxer=matroska \
      --enable-muxer=mov \
      --enable-muxer=null \
      \
      --disable-parsers \
      --enable-parser=h264 \
      --enable-parser=hevc \
      --enable-parser=vp8 \
      --enable-parser=vp9 \
      --enable-parser=av1 \
      --enable-parser=mpeg4video \
      --enable-parser=mpegaudio \
      --enable-parser=aac \
      --enable-parser=opus \
      --enable-parser=flac \
      --enable-parser=ac3 \
      \
      --disable-bsfs \
      --enable-bsf=h264_mp4toannexb \
      --enable-bsf=hevc_mp4toannexb \
      --enable-bsf=extract_extradata \
      --enable-bsf=aac_adtstoasc \
      \
      --disable-filters \
      --enable-filter=aresample \
      --enable-filter=anull \
      --enable-filter=null \
      --enable-filter=scale \
      --enable-filter=format \
      --enable-filter=vflip \
      --enable-filter=hflip \
      --enable-filter=transpose \
      \
      --extra-cflags="-I/build/prefix/include" \
      --extra-ldflags="-L/build/prefix/lib -static" \
    && make -j$(nproc) && make install

# ── 6. Strip & compress ──
RUN x86_64-w64-mingw32-strip /build/out/bin/ffmpeg.exe /build/out/bin/ffprobe.exe
RUN upx --best --lzma /build/out/bin/ffmpeg.exe /build/out/bin/ffprobe.exe || true
RUN ls -lh /build/out/bin/

DOCKERFILE

echo "=== Extracting binaries ==="
CONTAINER_ID=$(docker create "$IMAGE_NAME")
docker cp "$CONTAINER_ID:/build/out/bin/ffmpeg.exe" "$OUTPUT_DIR/ffmpeg.exe"
docker cp "$CONTAINER_ID:/build/out/bin/ffprobe.exe" "$OUTPUT_DIR/ffprobe.exe"
docker rm "$CONTAINER_ID" > /dev/null

echo ""
echo "=== Done ==="
ls -lh "$OUTPUT_DIR"/ff*.exe
