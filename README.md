<p align="center">
  <h1 align="center">EZ265</h1>
  <p align="center">
    <strong>One-click H.265 (HEVC) batch video converter for Windows</strong>
  </p>
  <p align="center">
    <a href="#features">Features</a> &bull;
    <a href="#installation">Installation</a> &bull;
    <a href="#usage">Usage</a> &bull;
    <a href="#build-from-source">Build</a> &bull;
    <a href="#license">License</a>
  </p>
  <p align="center">
    <img src="https://img.shields.io/badge/platform-Windows%2010%2F11-blue?logo=windows" alt="Platform">
    <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white" alt="Go">
    <img src="https://img.shields.io/badge/NVENC-GPU%20Accelerated-76B900?logo=nvidia&logoColor=white" alt="NVENC">
    <img src="https://img.shields.io/github/license/ServalC4t/EZ265" alt="License">
  </p>
</p>

---

EZ265 (H.265 一発変換) is a lightweight, portable Windows desktop app that converts video files to H.265/HEVC with minimal effort. Drag, drop, click — done.

## Features

| Feature | Description |
|---|---|
| **GPU Acceleration** | NVIDIA NVENC auto-detection with CPU (x265) fallback |
| **Drag & Drop** | Drop files directly onto the window |
| **Explorer Integration** | Right-click context menu: "Add to EZ265" / "Add & Start" |
| **Adjustable Compression** | Slider from 10% to 90% compression rate |
| **Batch Processing** | Queue multiple files, pause/resume, cancel individual jobs |
| **Safe Output** | Saves to a subfolder — originals are never overwritten |
| **Filename Control** | Optionally append `_h265` and/or compression rate to filename |
| **Bilingual UI** | Japanese / English (auto-detected from OS, switchable in-app) |
| **Low Priority Mode** | Encode in the background without impacting other tasks |
| **Auto Shutdown** | Optionally shut down PC after all jobs complete |
| **Single Instance** | Additional files are sent to the running instance via TCP IPC |
| **Portable** | No installation required — just unzip and run |

## Supported Formats

| Input | Extensions |
|---|---|
| Video | `.mp4` `.mkv` `.mov` `.avi` `.wmv` `.flv` `.m4v` `.ts` `.mts` `.m2ts` `.webm` |

Output is always H.265/HEVC in an MP4 container with `hvc1` tag and `faststart` flag.

## Installation

1. Download the latest release from [**Releases**](../../releases)
2. Extract the zip file anywhere
3. Run `h265conv.exe`

> **Requirements:**
> - Windows 10 or 11 (64-bit)
> - NVIDIA GPU recommended (for NVENC hardware encoding)
> - No GPU? No problem — falls back to CPU encoding automatically

## Usage

### Basic Workflow

```
1. Launch EZ265
2. Drag & drop video files onto the window (or click "+ Add Files")
3. Adjust compression rate with the slider (default: 50%)
4. Click "▶ Start"
5. Converted files appear in a "H265一発変換" subfolder
```

### Explorer Right-Click Menu

Register from inside the app with the **"Add Context Menu"** button:

- **Add to EZ265** — Adds the file to the queue (opens the app if not running)
- **Add to EZ265 & Start** — Adds and immediately starts encoding

### Settings

| Setting | Description | Default |
|---|---|---|
| Compression Rate | Target bitrate = Original × Rate | 50% |
| Append h265 | Add `_h265` to output filename | ON |
| Append Rate | Add `_50%` (etc.) to output filename | ON |
| Trash Original | Move source file to Recycle Bin after encoding | OFF |
| Low Priority | Run ffmpeg at idle CPU priority | OFF |
| Shutdown on Done | Shut down PC when all jobs complete | OFF |

Settings are saved automatically to `%APPDATA%\h265conv\settings.json`.

## Build from Source

### Prerequisites

- Go 1.21+ with CGO enabled
- MinGW-w64 (required for [lxn/walk](https://github.com/lxn/walk))
- `ffmpeg.exe` and `ffprobe.exe` in a `bin/` directory

### Steps

```bash
# Clone
git clone https://github.com/ServalC4t/EZ265.git
cd EZ265

# Generate Windows manifest resource
go install github.com/akavel/rsrc@latest
rsrc -manifest assets/h265conv.manifest -o h265conv_windows_amd64.syso

# Build
go build -ldflags="-H windowsgui -w -s" -trimpath -o h265conv.exe .
```

### Minimal ffmpeg Build (Docker)

Build stripped-down ffmpeg/ffprobe binaries (~3 MB each) with only the codecs needed:

```bash
# Requires Docker
bash build/build_ffmpeg_minimal.sh
```

This cross-compiles a minimal ffmpeg with:
- Decoders: H.264, H.265, VP9, common audio codecs
- Encoders: NVENC (hevc_nvenc) + x265 (libx265)
- Output: `bin/ffmpeg.exe` and `bin/ffprobe.exe`

## Architecture

```
h265conv/
├── main.go                          # Entry point, arg parsing, IPC
├── internal/
│   ├── encoder/
│   │   ├── ffmpeg.go                # FFmpeg wrapper (probe, encode, NVENC detection)
│   │   ├── job.go                   # Job model and status types
│   │   ├── queue.go                 # Serial job queue with pause/resume/cancel
│   │   ├── settings.go              # Settings persistence (%APPDATA%)
│   │   ├── priority_windows.go      # Process priority and window hiding
│   │   └── trash_windows.go         # SHFileOperation recycle bin
│   ├── gui/
│   │   └── mainwindow.go            # Walk-based Windows native GUI
│   ├── i18n/
│   │   └── i18n.go                  # Bilingual string table (JA/EN)
│   ├── ipc/
│   │   └── pipe.go                  # TCP single-instance IPC (127.0.0.1:19265)
│   └── registry/
│       └── contextmenu_windows.go   # Explorer context menu registration
├── assets/
│   └── h265conv.manifest            # DPI awareness + Common Controls v6
└── build/
    ├── package.ps1                  # PowerShell build + package script
    └── build_ffmpeg_minimal.sh      # Docker cross-compile for minimal ffmpeg
```

## Tech Stack

- **[Go](https://go.dev/)** — Fast, compiled, easy cross-compilation
- **[lxn/walk](https://github.com/lxn/walk)** — Native Windows GUI toolkit
- **[ffmpeg](https://ffmpeg.org/)** — Video encoding (NVENC + libx265)
- **Windows API** — Registry, SHFileOperation, process management

## License

[MIT](LICENSE) — Copyright 2026 ServalC4t

## Disclaimer

This software is provided "as is" without warranty of any kind. The author is not responsible for any damage caused by the use of this software. **Always back up your original files before batch conversion.**

---

<p align="center">
  Developed by <a href="https://github.com/ServalC4t">ServalC4t</a>
</p>
