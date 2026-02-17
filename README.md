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
- **🔧 Simple Configuration**: Easy-to-use YAML configuration (`go2nvr.yaml`).

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

Go2NVR uses `go2nvr.yaml` for configuration.

### Minimal Example

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

## Usage

Run the server:

```bash
./go2nvr
```

Access the dashboard at `http://localhost:1984`.

## Project Structure

- `cmd/`: Entry points.
- `internal/record/`: Core recording logic and API.
- `www/`: Custom web frontend source code.
- `pkg/`: Shared libraries and utilities.

## License

This project is based on [go2rtc](https://github.com/AlexxIT/go2rtc) and is licensed under the MIT License.
