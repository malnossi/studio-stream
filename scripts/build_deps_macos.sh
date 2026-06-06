#!/bin/bash
set -e

WORKSPACE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPS_DIST="${WORKSPACE_DIR}/deps-dist"
DEPS_DIST_X86="${WORKSPACE_DIR}/deps-dist-x86"
LAME_VERSION="3.100"
PA_ARCHIVE="pa_stable_v190700_20210406"

# Check if deps already built
if [ -f "${DEPS_DIST}/lib/libmp3lame.a" ] && [ -f "${DEPS_DIST}/lib/libopus.a" ]; then
    echo "Dependencies already built statically in ${DEPS_DIST}."
    exit 0
fi

echo "--- Building Static Dependencies for macOS (Universal) ---"
mkdir -p "${DEPS_DIST}"

# ---------- LAME universal ----------
echo "Building LAME..."
cd /tmp
curl -fsSLk -O https://downloads.sourceforge.net/project/lame/lame/${LAME_VERSION}/lame-${LAME_VERSION}.tar.gz || true
tar xf lame-${LAME_VERSION}.tar.gz
cd lame-${LAME_VERSION}

# Update config.sub/config.guess
curl -fsSL -o config.sub   'https://git.savannah.gnu.org/cgit/config.git/plain/config.sub'
curl -fsSL -o config.guess 'https://git.savannah.gnu.org/cgit/config.git/plain/config.guess'

# --- arm64 slice ---
./configure --prefix=${DEPS_DIST} \
  --host=aarch64-apple-darwin \
  --enable-static --disable-shared --disable-frontend \
  CFLAGS="-arch arm64 -O2" LDFLAGS="-arch arm64"
make -j$(sysctl -n hw.ncpu)
make install
cp ${DEPS_DIST}/lib/libmp3lame.a ${WORKSPACE_DIR}/libmp3lame_arm64.a

# --- x86_64 slice ---
make clean
./configure --prefix=${DEPS_DIST_X86} \
  --host=x86_64-apple-darwin \
  --enable-static --disable-shared --disable-frontend \
  CFLAGS="-arch x86_64 -O2" LDFLAGS="-arch x86_64"
make -j$(sysctl -n hw.ncpu)
make install
cp ${DEPS_DIST_X86}/lib/libmp3lame.a ${WORKSPACE_DIR}/libmp3lame_x86_64.a

# --- merge ---
lipo -create ${WORKSPACE_DIR}/libmp3lame_arm64.a ${WORKSPACE_DIR}/libmp3lame_x86_64.a \
  -output ${DEPS_DIST}/lib/libmp3lame.a
rm -f ${WORKSPACE_DIR}/libmp3lame_arm64.a ${WORKSPACE_DIR}/libmp3lame_x86_64.a

# ---------- PortAudio universal ----------
echo "Building PortAudio..."
cd /tmp
curl -fsSLk -O http://files.portaudio.com/archives/${PA_ARCHIVE}.tgz || true
tar xf ${PA_ARCHIVE}.tgz
cd portaudio

# --- arm64 slice ---
mkdir -p build-arm64 && cd build-arm64
cmake .. \
  -DCMAKE_INSTALL_PREFIX=${DEPS_DIST} \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
  -DCMAKE_OSX_ARCHITECTURES=arm64 \
  -DBUILD_SHARED_LIBS=OFF
make -j$(sysctl -n hw.ncpu)
make install
cp ${DEPS_DIST}/lib/libportaudio.a ${WORKSPACE_DIR}/libportaudio_arm64.a
cd ..

# --- x86_64 slice ---
mkdir -p build-x86 && cd build-x86
cmake .. \
  -DCMAKE_INSTALL_PREFIX=${DEPS_DIST_X86} \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
  -DCMAKE_OSX_ARCHITECTURES=x86_64 \
  -DBUILD_SHARED_LIBS=OFF
make -j$(sysctl -n hw.ncpu)
make install
cp ${DEPS_DIST_X86}/lib/libportaudio.a ${WORKSPACE_DIR}/libportaudio_x86_64.a
cd ..

# --- merge ---
lipo -create ${WORKSPACE_DIR}/libportaudio_arm64.a ${WORKSPACE_DIR}/libportaudio_x86_64.a \
  -output ${DEPS_DIST}/lib/libportaudio.a
rm -f ${WORKSPACE_DIR}/libportaudio_arm64.a ${WORKSPACE_DIR}/libportaudio_x86_64.a

# Cleanup shared libs
rm -f ${DEPS_DIST}/lib/*.dylib
rm -f ${DEPS_DIST_X86}/lib/*.dylib 2>/dev/null || true

# ---------- FDK-AAC universal ----------
echo "Building FDK-AAC..."
cd /tmp
curl -fsSLk -O https://downloads.sourceforge.net/project/opencore-amr/fdk-aac/fdk-aac-2.0.2.tar.gz || true
rm -rf fdk-aac-arm64 fdk-aac-x86
mkdir fdk-aac-arm64 fdk-aac-x86
tar xf fdk-aac-2.0.2.tar.gz -C fdk-aac-arm64 --strip-components=1
tar xf fdk-aac-2.0.2.tar.gz -C fdk-aac-x86 --strip-components=1

cd fdk-aac-arm64
./configure --prefix=${DEPS_DIST} --host=aarch64-apple-darwin --enable-static --disable-shared CFLAGS="-arch arm64 -O2" CXXFLAGS="-arch arm64 -O2" LDFLAGS="-arch arm64"
make -j$(sysctl -n hw.ncpu) && make install
cp ${DEPS_DIST}/lib/libfdk-aac.a ${WORKSPACE_DIR}/libfdkaac_arm64.a

cd ../fdk-aac-x86
./configure --prefix=${DEPS_DIST_X86} --host=x86_64-apple-darwin --enable-static --disable-shared CFLAGS="-arch x86_64 -O2" CXXFLAGS="-arch x86_64 -O2" LDFLAGS="-arch x86_64"
make -j$(sysctl -n hw.ncpu) && make install
cp ${DEPS_DIST_X86}/lib/libfdk-aac.a ${WORKSPACE_DIR}/libfdkaac_x86_64.a

lipo -create ${WORKSPACE_DIR}/libfdkaac_arm64.a ${WORKSPACE_DIR}/libfdkaac_x86_64.a -output ${DEPS_DIST}/lib/libfdk-aac.a
rm -f ${WORKSPACE_DIR}/libfdkaac_*.a

# ---------- Ogg universal ----------
echo "Building Ogg..."
cd /tmp
curl -fsSLk -O https://downloads.xiph.org/releases/ogg/libogg-1.3.5.tar.gz || true
rm -rf libogg-arm64 libogg-x86
mkdir libogg-arm64 libogg-x86
tar xf libogg-1.3.5.tar.gz -C libogg-arm64 --strip-components=1
tar xf libogg-1.3.5.tar.gz -C libogg-x86 --strip-components=1

cd libogg-arm64
sed -i '' 's/-force_cpusubtype_ALL//g' configure || true
./configure --prefix=${DEPS_DIST} --host=aarch64-apple-darwin --enable-static --disable-shared CFLAGS="-arch arm64 -O2" LDFLAGS="-arch arm64"
make -j$(sysctl -n hw.ncpu) && make install
cp ${DEPS_DIST}/lib/libogg.a ${WORKSPACE_DIR}/libogg_arm64.a

cd ../libogg-x86
sed -i '' 's/-force_cpusubtype_ALL//g' configure || true
./configure --prefix=${DEPS_DIST_X86} --host=x86_64-apple-darwin --enable-static --disable-shared CFLAGS="-arch x86_64 -O2" LDFLAGS="-arch x86_64"
make -j$(sysctl -n hw.ncpu) && make install
cp ${DEPS_DIST_X86}/lib/libogg.a ${WORKSPACE_DIR}/libogg_x86_64.a

lipo -create ${WORKSPACE_DIR}/libogg_arm64.a ${WORKSPACE_DIR}/libogg_x86_64.a -output ${DEPS_DIST}/lib/libogg.a
rm -f ${WORKSPACE_DIR}/libogg_*.a

# ---------- Vorbis universal ----------
echo "Building Vorbis..."
cd /tmp
curl -fsSL -O https://downloads.xiph.org/releases/vorbis/libvorbis-1.3.7.tar.gz || true
rm -rf libvorbis-arm64 libvorbis-x86
mkdir libvorbis-arm64 libvorbis-x86
tar xf libvorbis-1.3.7.tar.gz -C libvorbis-arm64 --strip-components=1
tar xf libvorbis-1.3.7.tar.gz -C libvorbis-x86 --strip-components=1

cd libvorbis-arm64
sed -i '' 's/-force_cpusubtype_ALL//g' configure || true
./configure --prefix=${DEPS_DIST} --host=aarch64-apple-darwin --enable-static --disable-shared --with-ogg=${DEPS_DIST} CFLAGS="-arch arm64 -O2" LDFLAGS="-arch arm64"
make -j$(sysctl -n hw.ncpu) && make install
cp ${DEPS_DIST}/lib/libvorbis.a ${WORKSPACE_DIR}/libvorbis_arm64.a
cp ${DEPS_DIST}/lib/libvorbisenc.a ${WORKSPACE_DIR}/libvorbisenc_arm64.a

cd ../libvorbis-x86
sed -i '' 's/-force_cpusubtype_ALL//g' configure || true
./configure --prefix=${DEPS_DIST_X86} --host=x86_64-apple-darwin --enable-static --disable-shared --with-ogg=${DEPS_DIST_X86} CFLAGS="-arch x86_64 -O2" LDFLAGS="-arch x86_64"
make -j$(sysctl -n hw.ncpu) && make install
cp ${DEPS_DIST_X86}/lib/libvorbis.a ${WORKSPACE_DIR}/libvorbis_x86_64.a
cp ${DEPS_DIST_X86}/lib/libvorbisenc.a ${WORKSPACE_DIR}/libvorbisenc_x86_64.a

lipo -create ${WORKSPACE_DIR}/libvorbis_arm64.a ${WORKSPACE_DIR}/libvorbis_x86_64.a -output ${DEPS_DIST}/lib/libvorbis.a
lipo -create ${WORKSPACE_DIR}/libvorbisenc_arm64.a ${WORKSPACE_DIR}/libvorbisenc_x86_64.a -output ${DEPS_DIST}/lib/libvorbisenc.a
rm -f ${WORKSPACE_DIR}/libvorbis*.a

# ---------- Opus universal ----------
echo "Building Opus..."
cd /tmp
curl -fsSL -O https://downloads.xiph.org/releases/opus/opus-1.4.tar.gz || true
rm -rf opus-arm64 opus-x86
mkdir opus-arm64 opus-x86
tar xf opus-1.4.tar.gz -C opus-arm64 --strip-components=1
tar xf opus-1.4.tar.gz -C opus-x86 --strip-components=1

cd opus-arm64
./configure --prefix=${DEPS_DIST} --host=aarch64-apple-darwin --enable-static --disable-shared CFLAGS="-arch arm64 -O2" LDFLAGS="-arch arm64"
make -j$(sysctl -n hw.ncpu) && make install
cp ${DEPS_DIST}/lib/libopus.a ${WORKSPACE_DIR}/libopus_arm64.a

cd ../opus-x86
./configure --prefix=${DEPS_DIST_X86} --host=x86_64-apple-darwin --enable-static --disable-shared CFLAGS="-arch x86_64 -O2" LDFLAGS="-arch x86_64"
make -j$(sysctl -n hw.ncpu) && make install
cp ${DEPS_DIST_X86}/lib/libopus.a ${WORKSPACE_DIR}/libopus_x86_64.a

lipo -create ${WORKSPACE_DIR}/libopus_arm64.a ${WORKSPACE_DIR}/libopus_x86_64.a -output ${DEPS_DIST}/lib/libopus.a
rm -f ${WORKSPACE_DIR}/libopus_*.a

echo "Done building macOS static dependencies!"
