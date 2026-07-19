# go2nvr

<p align="center">
  <a href="https://github.com/57Darling02/go2nvr/actions/workflows/build.yml" target="_blank">
    <img style="display: inline" src="https://img.shields.io/github/actions/workflow/status/57Darling02/go2nvr/build.yml?style=flat-square&logo=github" alt="go2nvr build status">
  </a>
  <a href="https://github.com/57Darling02/go2nvr/stargazers" target="_blank">
    <img style="display: inline" src="https://img.shields.io/github/stars/57Darling02/go2nvr?style=flat-square&logo=github" alt="go2nvr GitHub stars">
  </a>
  <a href="./LICENSE" target="_blank">
    <img style="display: inline" src="https://img.shields.io/github/license/57Darling02/go2nvr?style=flat-square" alt="go2nvr license">
  </a>
</p>

**go2nvr is a lightweight, high-performance NVR built on [go2rtc](https://github.com/AlexxIT/go2rtc).**

It keeps go2rtc's multi-protocol streaming, low-latency preview, and simple deployment model, then adds the NVR features most small systems need: a Web UI, on-demand recording, motion-triggered recording, pre-record buffering, playback, file management, retention cleanup, and TTS push-to-camera audio.

go2nvr is not trying to be a heavy AI surveillance platform. It sits between a media gateway and a full monitoring suite: light like go2rtc, but with practical recording management.

## Highlights

- **Built on go2rtc**: RTSP, WebRTC, HLS, ONVIF, and other go2rtc stream capabilities.
- **Web dashboard**: live preview, stream management, recording rules, playback, and system settings.
- **Manual recording**: start and stop recordings from the Web UI or HTTP API.
- **Motion-triggered recording**: simple built-in frame-difference detection.
- **Pre-record buffer**: keep seconds of video before the trigger event.
- **Recording library**: browse, play, download, and delete recordings.
- **Retention policy**: automatically remove old recordings by day count.
- **TTS push**: convert text to speech and push it to IP Webcam or go2rtc backchannel targets.
- **Small deployment footprint**: one process, no required Docker, database, queue, or external service for the core path.

Optional tools such as FFmpeg can still be used when a stream needs special protocol or codec handling.

## When To Use It

go2nvr fits small servers, NAS boxes, ARM devices, edge gateways, and low-power Linux hosts where you want:

- a lightweight NVR instead of a full surveillance stack;
- RTSP, ONVIF, IP Webcam, phone camera, or mixed stream sources;
- live preview, recording, playback, and file management from a browser;
- a simple setup without Docker, a database, or AI inference services.

## Screenshots

Dashboard overview

![Dashboard overview](docs/screenshots/dashboard.png)

Live control and TTS

![Live control and TTS](docs/screenshots/live-control-tts.png)

Recordings

![Recordings](docs/screenshots/recordings.png)

Recording rule

![Recording rule](docs/screenshots/record-rule.png)

System configuration

![System configuration](docs/screenshots/config-system.png)

## Quick Start

Download the binary for your platform:

- [Latest release](https://github.com/57Darling02/go2nvr/releases/latest)

Linux, macOS, or FreeBSD example:

```bash
chmod +x ./go2nvr_linux_amd64
./go2nvr_linux_amd64
```

Windows:

```powershell
.\go2nvr_win64.exe
```

Open the Web UI:

```text
http://<device-ip>:1984
```

By default, go2nvr reads `go2nvr.yaml` from the current directory. If the file does not exist, start go2nvr and save the configuration from the Web UI.

Common first steps:

- add camera streams;
- check live preview;
- set recording directory and retention days;
- enable prebuffer and trigger rules for selected streams;
- browse, play, download, or delete recordings;
- push TTS audio from Live Control.

## Minimal Config

```yaml
streams:
  camera1:
    - rtsp://user:password@192.168.1.10:554/stream1

api:
  listen: ":1984"

record:
  dir: ./records
  retention: 7
  limits:
    memory_mb: 256
    prebuffer_mb: 32
    writer_queue_mb: 16
    snapshot_workers: 1
  rules:
    - src: camera1
      prebuffer: 10
      trigger_id: 1
      trigger_interval: 500
      trigger_params:
        threshold: 14
        post_sec: 10
        min_hits: 1
```

Use a specific config file:

```bash
./go2nvr -c go2nvr.yaml
```

Build from source:

```bash
git clone https://github.com/57Darling02/go2nvr.git
cd go2nvr
./scripts/build-go2nvr.sh -o go2nvr
```

Source builds require Go 1.24+, Node.js 22+, and pnpm 10. The dashboard is generated during the build and is intentionally not committed to Git.

To host go2nvr below a reverse-proxy prefix, set `api.base_path` (for example, `/nvr`). The dashboard derives its API, WebSocket, router, and asset URLs from that path.

## Recording

The Web UI is the recommended way to manage recording settings. For automated deployment, use YAML:

```yaml
record:
  dir: ./records
  retention: 7
  limits:
    memory_mb: 256
    prebuffer_mb: 32
    writer_queue_mb: 16
    snapshot_workers: 1
  rules:
    - src: camera1
      prebuffer: 10
      trigger_id: 1
      trigger_interval: 500
      trigger_params:
        threshold: 14
        post_sec: 10
        min_hits: 1
```

Useful APIs:

```text
GET    /api/record
POST   /api/record?src=camera1&action=start
POST   /api/record?src=camera1&action=stop
GET    /api/record/rules
GET    /api/record/triggers
GET    /api/record/file?path=...
DELETE /api/record/file?path=...
```

The advanced `limits` block is YAML-only; it is not exposed in the UI or record config API. New recordings are stored under `records/streams/<sha256(source)>/...`; existing pre-upgrade recordings are intentionally not migrated or listed. See [`internal/record`](internal/record/README.md) for the state fields, safe logical file paths, and retention behavior.

## TTS Push

go2nvr can convert text to speech and push audio to a camera channel.

Supported targets:

- `ipwebcam` / `wss`: IP Webcam WebSocket audio input.
- `backchannel` / `stream`: go2rtc two-way audio backchannel.

Example:

```bash
curl -X POST "http://127.0.0.1:1984/api/ttspush/push" \
  --data-urlencode "text=Please leave the area" \
  --data-urlencode "target_type=ipwebcam" \
  --data-urlencode "url=ws://192.168.1.20:8080/audioin.wav"
```

See [`internal/ttspush`](internal/ttspush/README.md) for parameters and target options.

## How It Compares

| Project | Focus | Difference in go2nvr |
| --- | --- | --- |
| go2rtc | Media gateway | Adds recording, triggers, playback, file management, and TTS. |
| MediaMTX | Media server | Adds NVR-oriented Web management and recording workflows. |
| Frigate | AI NVR | Much lighter, without required AI inference or Docker. |
| Shinobi / ZoneMinder | Full surveillance suite | Smaller scope for users who need core NVR features. |
| tinyCam Monitor | Mobile monitoring app | Server-first Web UI and recording management. |

## Project Status

go2nvr is usable today, but still evolving. The core stream, Web UI, recording, trigger, playback, and TTS paths are available. Feedback is especially welcome for NAS, ARM, edge device, multi-camera, and lightweight deployment scenarios.

## Credits

go2nvr is built on [AlexxIT/go2rtc](https://github.com/AlexxIT/go2rtc) and inherits its excellent streaming protocol support. TTS functionality uses capabilities from [edge-tts-go](https://github.com/difyz9/edge-tts-go).

## License

See [LICENSE](LICENSE).
