# StudioStream

**StudioStream** is a powerful, standalone live audio broadcasting engine for Icecast, built entirely with Go, [Raylib](https://www.raylib.com/), and [Raygui](https://github.com/raysan5/raygui).

## What is StudioStream?

StudioStream acts as a reliable, seamless medium connecting your audio source directly to your listeners across the globe. By completely dropping heavy web-based frameworks like Wails or Vue 3, StudioStream achieves an incredibly lightweight, ultra-performant, and visually stunning native desktop experience.

## Features

- **Robust Audio Engine:** A highly performant broadcasting engine written in Go.
- **Icecast Compatibility:** Seamlessly streams to Icecast servers with automatic Handshake and TLS support.
- **Native UI:** A fully custom, dark-mode GUI engineered with Raylib and Raygui using the Roboto font, giving you the feel of professional broadcasting equipment.
- **Comprehensive Codec Support:** Statically links several industry-standard C libraries to handle diverse audio formats with zero runtime dependency headaches:
  - PortAudio
  - LAME
  - FDK-AAC
  - Opus
  - Ogg
  - Vorbis
- **Multiple Configurations:** Save and load various stream settings using JSON configurations dynamically.
- **Cross-Platform:** One codebase, statically compiled for macOS, Windows, and Linux!

## Building the Project

StudioStream uses a `Makefile` to simplify building dependencies and the final application into standalone binaries.

### macOS

1. Install Homebrew and the required build tools natively on your Mac.
2. Build the application:
   ```bash
   make build-macos
   ```
   This command natively downloads and statically links all C-dependencies and generates the fully packaged `StudioStream.app` bundle located in `target/bin/StudioStream.app`.

### Windows

Building for Windows is fully supported natively on macOS via Cross-Compilation using `mingw-w64` and `zig cc`.

1. Ensure you have `mingw-w64`, `zig`, and `go-winres` installed.
2. Build the application:
   ```bash
   make build-windows
   ```
   This command cross-compiles all the complex C-dependencies natively with `mingw-w64`, intercepts threading flags, and uses `zig cc` as the Go compiler to generate a fully static, standalone executable `StudioStream.exe` (with embedded icons via `go-winres`) in `target/bin/StudioStream.exe`. No external DLLs required!

### Linux

Building for Linux compiles a standalone binary.

1. Ensure you have your mandatory Linux system dependencies and build tools installed. On Debian/Ubuntu-based systems, you can install them via:
   ```bash
   sudo apt-get update
   sudo apt-get install -y build-essential pkg-config libasound2-dev curl tar wget
   ```
2. Build the application:
   ```bash
   make build-linux
   ```
   This command automatically downloads and compiles the static C dependencies (PortAudio, LAME, FDK-AAC, Opus, Ogg, Vorbis), and compiles the Linux binary using CGO, placing the standalone `StudioStream-linux` binary in `target/bin/`.

## Contributing

Contributions are always welcome! Since StudioStream compiles with multiple C-dependencies, setting up the development environment might seem daunting, but it's fully automated:

1. Fork the project.
2. Ensure you have the required C compilers and build tools for your platform.
3. Run the application locally with `go run main.go`.
4. Submit a Pull Request with your feature or bug fix.

When contributing, please ensure your code follows the Go standard formatting (`go fmt`).

## License

StudioStream is released under the [MIT License](LICENSE). 

Please note that compiling StudioStream statically links multiple third-party libraries, including LAME (LGPL) and FDK-AAC (Fraunhofer). The MIT license applies to the StudioStream application source code itself. Ensure you comply with the respective licenses of all statically linked dependencies if you decide to distribute compiled binaries.
