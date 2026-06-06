#!/bin/bash
set -e

WORKSPACE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPS_DIST="${WORKSPACE_DIR}/deps-dist-linux"
LAME_VERSION="3.100"
PA_ARCHIVE="pa_stable_v190700_20210406"

# Check if deps already built
if [ -f "${DEPS_DIST}/lib/libmp3lame.a" ] && [ -f "${DEPS_DIST}/lib/libportaudio.a" ]; then
    echo "Dependencies already built statically in ${DEPS_DIST}."
    exit 0
fi

echo "--- Building Static Dependencies for Linux ---"
mkdir -p "${DEPS_DIST}"

# ---------- LAME ----------
echo "Building LAME..."
cd /tmp
curl -fsSLk -O https://downloads.sourceforge.net/project/lame/lame/${LAME_VERSION}/lame-${LAME_VERSION}.tar.gz || true
tar xf lame-${LAME_VERSION}.tar.gz
cd lame-${LAME_VERSION}
./configure --prefix="${DEPS_DIST}" --enable-static --disable-shared --disable-frontend
make -j$(nproc 2>/dev/null || echo 4)
make install

# ---------- PortAudio ----------
echo "Building PortAudio..."
cd /tmp
curl -fsSLk -O http://files.portaudio.com/archives/${PA_ARCHIVE}.tgz || true
tar xf ${PA_ARCHIVE}.tgz
cd portaudio
./configure --prefix="${DEPS_DIST}" --enable-static --disable-shared --with-alsa --with-jack=no
make -j$(nproc 2>/dev/null || echo 4)
make install

# ---------- FDK-AAC ----------
echo "Building FDK-AAC..."
cd /tmp
curl -fsSLk -O https://downloads.sourceforge.net/project/opencore-amr/fdk-aac/fdk-aac-2.0.2.tar.gz || true
tar xf fdk-aac-2.0.2.tar.gz
cd fdk-aac-2.0.2
./configure --prefix="${DEPS_DIST}" --enable-static --disable-shared
make -j$(nproc 2>/dev/null || echo 4)
make install

# ---------- Ogg ----------
echo "Building Ogg..."
cd /tmp
curl -fsSLk -O https://downloads.xiph.org/releases/ogg/libogg-1.3.5.tar.gz || true
tar xf libogg-1.3.5.tar.gz
cd libogg-1.3.5
./configure --prefix="${DEPS_DIST}" --enable-static --disable-shared
make -j$(nproc 2>/dev/null || echo 4)
make install

# ---------- Vorbis ----------
echo "Building Vorbis..."
cd /tmp
curl -fsSLk -O https://downloads.xiph.org/releases/vorbis/libvorbis-1.3.7.tar.gz || true
tar xf libvorbis-1.3.7.tar.gz
cd libvorbis-1.3.7
./configure --prefix="${DEPS_DIST}" --enable-static --disable-shared --with-ogg="${DEPS_DIST}"
make -j$(nproc 2>/dev/null || echo 4)
make install

# ---------- Opus ----------
echo "Building Opus..."
cd /tmp
curl -fsSLk -O https://downloads.xiph.org/releases/opus/opus-1.4.tar.gz || true
tar xf opus-1.4.tar.gz
cd opus-1.4
./configure --prefix="${DEPS_DIST}" --enable-static --disable-shared
make -j$(nproc 2>/dev/null || echo 4)
make install

# ---------- Opusfile ----------
echo "Building Opusfile..."
cd /tmp
curl -fsSLk -O https://downloads.xiph.org/releases/opus/opusfile-0.12.tar.gz || true
tar xf opusfile-0.12.tar.gz
cd opusfile-0.12
# Need to supply PKG_CONFIG_PATH and CFLAGS so it finds ogg and opus
PKG_CONFIG_PATH="${DEPS_DIST}/lib/pkgconfig" CFLAGS="-I${DEPS_DIST}/include" ./configure --prefix="${DEPS_DIST}" --enable-static --disable-shared --disable-http
make -j$(nproc 2>/dev/null || echo 4)
make install

echo "Done building Linux static dependencies!"
