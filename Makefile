# Makefile for Atheer

WORKSPACE_DIR := $(shell pwd)
DEPS_DIST := $(WORKSPACE_DIR)/deps-dist
DEPS_DIST_LINUX := $(WORKSPACE_DIR)/deps-dist-linux
DEPS_DIST_WINDOWS := $(WORKSPACE_DIR)/deps-dist-windows

# Check OS to apply specific CI-like static linking logic
UNAME_S := $(shell uname -s)

ifeq ($(UNAME_S),Darwin)
	# macOS Static Linking Configuration
	CGO_CFLAGS  ?= -I$(DEPS_DIST)/include -I$(DEPS_DIST)/include/fdk-aac -I$(DEPS_DIST)/include/opus
	CGO_LDFLAGS ?= -L$(DEPS_DIST)/lib -lportaudio -lmp3lame -lfdk-aac -lopus -logg -lvorbis -lvorbisenc -framework CoreAudio -framework AudioToolbox -framework AudioUnit -framework CoreFoundation -Wl,-w
	PKG_CONFIG_PATH ?= $(DEPS_DIST)/lib/pkgconfig
	
	# Set macOS deployment target dynamically to match the local OS version
	MACOSX_DEPLOYMENT_TARGET ?= $(shell sw_vers -productVersion | cut -d. -f1-2)
	export MACOSX_DEPLOYMENT_TARGET
else
	# Fallback/Linux/Windows dynamic/static mixing (can be extended)
	CGO_CFLAGS  ?= -I/opt/homebrew/include -I/opt/homebrew/include/fdk-aac -I/opt/homebrew/include/opus
	CGO_LDFLAGS ?= -L/opt/homebrew/lib -lportaudio -lmp3lame -lfdk-aac -lopus -logg -lvorbis -lvorbisenc -Wl,-w
endif

# Export CGO variables to all child processes spawned by make targets
export CGO_CFLAGS
export CGO_LDFLAGS
export PKG_CONFIG_PATH

TARGET_DIR := $(WORKSPACE_DIR)/target

# Phony targets
.PHONY: all clean build-macos build-windows deps-windows

all: build-macos

# ==========================================
# macOS App Bundle Packaging
# ==========================================
build-macos:
	@echo "Building StudioStream for macOS..."
	@mkdir -p $(TARGET_DIR)/bin/StudioStream.app/Contents/MacOS
	@mkdir -p $(TARGET_DIR)/bin/StudioStream.app/Contents/Resources
	
	@echo "Compiling binary..."
	CGO_ENABLED=1 go build -o $(TARGET_DIR)/bin/StudioStream.app/Contents/MacOS/StudioStream main.go
	
	@echo "Copying Info.plist..."
	@cp $(TARGET_DIR)/darwin/Info.plist $(TARGET_DIR)/bin/StudioStream.app/Contents/
	
	@echo "Copying icon..."
	@cp $(TARGET_DIR)/darwin/appicon.icns $(TARGET_DIR)/bin/StudioStream.app/Contents/Resources/
	
	@echo "Done! macOS App Bundle located at $(TARGET_DIR)/bin/StudioStream.app"

# ==========================================
# Windows Packaging (zig cc + go-winres)
# ==========================================
deps-windows:
	@echo "Building static C dependencies for Windows..."
	@cd $(WORKSPACE_DIR)/scripts && chmod +x build_deps_windows.sh && ./build_deps_windows.sh

build-windows: deps-windows
	@echo "Generating Windows resources (icons and manifests)..."
	go run github.com/tc-hib/go-winres@latest make --in target/windows/winres.json
	@echo "Compiling for Windows using zig cc..."
	CC="$(WORKSPACE_DIR)/scripts/zig-cc-wrapper.sh" CXX="$(WORKSPACE_DIR)/scripts/zig-cxx-wrapper.sh" CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
	PKG_CONFIG_PATH="$(DEPS_DIST_WINDOWS)/lib/pkgconfig:$$PKG_CONFIG_PATH" \
	CGO_CFLAGS="-I$(DEPS_DIST_WINDOWS)/include" \
	CGO_LDFLAGS="-static -L$(DEPS_DIST_WINDOWS)/lib -lportaudio -lmp3lame -lfdk-aac -lopusfile -lopus -lvorbisenc -lvorbis -logg -lwinmm -lole32 -luuid -ldsound -lsetupapi -static-libgcc -static-libstdc++" \
	go build -ldflags="-H windowsgui -s -w" -o target/bin/StudioStream.exe main.go
	@echo "Cleaning up generated syso file..."
	rm -f rsrc_*.syso
	@echo "Done! Windows executable located at target/bin/StudioStream.exe"

# ==========================================
# Linux Packaging (AppImage)
# ==========================================
build-linux:
	@echo "Building StudioStream for Linux..."
	@mkdir -p build/AppDir/usr/bin
	@mkdir -p build/AppDir/usr/share/applications
	@mkdir -p build/AppDir/usr/share/icons/hicolor/256x256/apps
	
	@echo "Compiling Linux binary..."
	CGO_ENABLED=1 go build -o build/AppDir/usr/bin/studiostream main.go
	
	@echo "Copying AppImage resources..."
	cp packaging/appimage/studiostream.desktop build/AppDir/studiostream.desktop
	cp target/darwin/appicon.iconset/icon_256x256.png build/AppDir/studiostream.png
	cp packaging/appimage/studiostream.desktop build/AppDir/usr/share/applications/
	cp target/darwin/appicon.iconset/icon_256x256.png build/AppDir/usr/share/icons/hicolor/256x256/apps/studiostream.png
	
	@echo "Generating AppImage..."
	@if [ ! -f packaging/appimage/appimagetool-x86_64.AppImage ]; then \
		echo "Downloading appimagetool..."; \
		wget -qO packaging/appimage/appimagetool-x86_64.AppImage https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage; \
	fi
	chmod +x packaging/appimage/appimagetool-x86_64.AppImage
	./packaging/appimage/appimagetool-x86_64.AppImage build/AppDir target/bin/StudioStream-x86_64.AppImage
	@echo "Done! Linux AppImage located at target/bin/StudioStream-x86_64.AppImage"

clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR)
	rm -rf $(TARGET_DIR)/bin/*
