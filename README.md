# Go2NVR

**Go2NVR** is a high-performance Network Video Recorder (NVR) solution built on top of the powerful [go2rtc](https://github.com/AlexxIT/go2rtc) streaming server.

It extends the core streaming capabilities with a native, deep-integrated recording engine and a custom management dashboard, making it a complete solution for home and professional video surveillance.

## Key Features

- **🚀 Advanced Streaming Core**: Inherits all the robust streaming features of go2rtc (RTSP, WebRTC, MSE, HLS, HomeKit, etc.).
- **📹 Native Recording Engine**: 
  - Integrated directly into the core (no external scripts required).
  - Configurable recording rules (continuous, motion-based, etc.).
  - Automatic retention management (days-based cleanup).
  - High-performance writing directly to disk.
- **🖥️ Enhanced Web Dashboard**: 
  - A modern, custom-built web interface for managing cameras.
  - Built-in recordings browser and player.
  - Real-time system status monitoring.
- **🔧 Flexible Configuration**: 
  1. **Web UI**: Configure most features directly in the browser.
  2. **YAML**: Full access to advanced features via `go2nvr.yaml`.

## Installation

### From Source

Requirements: [Go](https://go.dev/) 1.24+

```bash
# Clone the repository
git clone https://github.com/57Darling02/go2nvr
cd go2nvr

# Build the binary
go build -ldflags "-s -w" -o go2nvr
```

## Configuration

Go2NVR offers two ways to configure your system.

### 1. Web UI (Recommended)

For most users, the Web UI provides all necessary controls.
Access the dashboard at `http://localhost:1984`.
- **Add Streams**: Manage your camera connections.
- **Recording Rules**: Set up continuous or motion-based recording.
- **Playback**: Browse and view recorded footage.

### 2. Advanced YAML Configuration

For advanced users requiring the full power of the underlying streaming engine, you can edit the `go2nvr.yaml` file. This file supports **all configuration options** available in [go2rtc](https://github.com/AlexxIT/go2rtc).

You can edit this file directly or via the **Config Editor** in the Web UI.

> **Note**: After modifying the configuration via YAML (file or Web UI), you must **restart the server** for changes to take effect.

Refer to the [go2rtc Configuration Documentation](https://go2rtc.org/#configuration) for a complete list of supported streams, protocols, and advanced settings.

#### Minimal Example (`go2nvr.yaml`)

```yaml
streams:
  # Define your camera streams here
  camera1: rtsp://admin:password@192.168.1.100:554/stream1

record:
  dir: recordings       # Directory to save recordings
  retention: 7          # Keep recordings for 7 days
  rules:
    - src: camera1      # Source stream name (must match streams section)
      mode: all         # Recording mode: all (continuous)
      segment: 60       # Segment duration in seconds
```

## Recording System

The recording module turns the streaming server into a fully-fledged NVR. It operates on a **dual-mode philosophy**:

### 1. Universal Manual Recording
Any stream can be recorded instantly via the API or Web UI button, **without any prior configuration**. This is useful for capturing specific events on demand.

### 2. Automated Strategies
You can define persistent recording rules for your cameras.

| Mode | Description |
| :--- | :--- |
| **Always (`all`)** | **Continuous 24/7 Recording**. The system records everything. Files are automatically segmented based on the `segment` duration (default: 600s). |
| **Motion (`motion`)** | **Smart Event Recording**. The system buffers video in memory and only writes to disk when motion is detected. <br>• **Pre-buffer**: Captures seconds *before* the motion event (never miss the start).<br>• **Post-buffer**: Continues recording for a set time after motion stops. |

### Storage & Retention
- **Structure**: Recordings are organized by `stream_name/YYYY-MM-DD/HH-MM-SS.mp4`.
- **Cleanup**: An automatic maintenance task runs hourly to delete recordings older than your configured `retention` period (in days).

## Usage

Run the server:

```bash
./go2nvr
```

## Project Structure

- `cmd/`: Entry points.
- `internal/record/`: Core recording logic and API.
- `www/`: Custom web frontend source code.
- `pkg/`: Shared libraries and utilities.

## License

This project is based on [go2rtc](https://github.com/AlexxIT/go2rtc) and is licensed under the MIT License.
