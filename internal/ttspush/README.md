# TTS Push (Streaming)

This module is now streaming-only and no-disk:

- `generate.go`: `edge-tts-go` stream -> optional ffmpeg stream transcode
- `target/ipwebcam.go`: persistent WSS workers with bounded queue + idle cleanup
- `target/backchannel.go`: stream.Play backchannel from live stream reader
- `ttspush.go`: API request parsing and target dispatch

## API

Only one endpoint is exposed:

- `POST /api/ttspush/push`

## Request Params

Common:

- `text` (required)
- `voice` (default `zh-CN-XiaoxiaoNeural`)
- `rate` (default `+0%`)
- `pitch` (default `+0Hz`)
- `volume` (default `+0%`)
- `proxy` (default `HTTPS_PROXY`)
- `connect_timeout` (default `10`)
- `receive_timeout` (default `90`)

Target: `ipwebcam` / `wss`

- `target_type`: `ipwebcam` or `wss`
- `url` (or `target`): websocket URL (supports `ws://user:pass@...`)
- `sample_rate` (default `24000`)
- `chunk_ms` (default `40`)
- `realtime` (default `true`)
- `insecure_tls` (default `false`)

Target: `backchannel` / `stream`

- `target_type`: `backchannel` or `stream`
- `dst` (or `target`): stream name
- `format`: currently `wav` only

## Notes

- The pipeline is fully in-memory streaming, no file write.
- `ipwebcam` has per-URL bounded queue (`16`) and idle client cleanup (`5m`).
- `realtime=true` sends at audio pace; `false` sends as fast as possible.
