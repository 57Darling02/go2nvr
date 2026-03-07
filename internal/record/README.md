# Record Module

`internal/record` provides local recording, prebuffer, trigger-driven auto start/stop, and file management APIs.

## Architecture

- `record.go`: module init, recorder lifecycle, HTTP APIs.
- `recorder.go`: RTP ingest, prebuffer queue, MP4/MJPEG write, snapshot/thumbnail pipeline.
- `trigger_bind.go`: bridge between recording layer and trigger manager.

Trigger-specific architecture, detector contract, lifecycle, and contributor quickstart are documented in:

- [`internal/record/trigger/README.md`](./trigger/README.md)

## Config (YAML)

```yaml
record:
  dir: ./records
  retention: 7
  rules:
    - src: camera1
      prebuffer: 10
      trigger_id: 1
      trigger_interval: 250
      trigger_params:
        threshold: 14
        post_sec: 10
        min_hits: 1
```

### Rule fields

- `src`: stream name.
- `prebuffer`: seconds to keep before recording starts.
- `trigger_id`: detector id. `<=0` means trigger disabled.
- `trigger_interval`: polling interval in milliseconds. Default: `250`.
- `trigger_params`: detector-specific params (`map[string]any`).

### Module fields

- `dir`: recording root directory. Empty or `/` falls back to `./records`.
- `retention`: days to keep date folders. `<=0` falls back to `7`.

## HTTP APIs

### 1) Recording state and control

- `GET /api/record`
  - list all stream states.
- `GET /api/record?src={stream}`
  - show one stream state.
- `POST /api/record?src={stream}&action=start|stop`
  - manual start/stop.

State payload example:

```json
{
  "name": "camera1",
  "status": "recording",
  "file": "records/camera1/2026-03-07/09-12-30.mp4",
  "duration": "43s",
  "prebuffer": 10,
  "trigger_id": 1,
  "trigger_key": "simple_diff",
  "trigger_name": "Simple Diff"
}
```

`status` meanings:

- `recording`: actively writing a file.
- `idle`: recorder exists but not writing.
- `stopped`: no recorder attached.

### 2) Rule management

- `GET /api/record/rules`
  - list all rules.
- `GET /api/record/rules?src={stream}`
  - get one rule.
- `POST /api/record/rules`
  - upsert a rule.
- `DELETE /api/record/rules?src={stream}`
  - remove a rule and stop its trigger worker.

Upsert example:

```json
{
  "src": "camera1",
  "prebuffer": 10,
  "trigger_id": 1,
  "trigger_interval": 250,
  "trigger_params": {
    "threshold": 16,
    "post_sec": 12,
    "min_hits": 2
  }
}
```

Notes:

- Rule is persisted first. If stream is temporarily unavailable, API still returns `200` and may include `attach_error`.
- `src` is normalized (trailing query part after `?` is removed).

### 3) Trigger metadata

- `GET /api/record/triggers`
  - list all registered detectors and parameter schema.

Example:

```json
[
  {
    "id": 1,
    "key": "simple_diff",
    "name": "Simple Diff",
    "params": [
      { "key": "threshold", "type": "number", "default": 14, "min": 1, "max": 255, "tip": "Average grayscale diff threshold treated as motion." },
      { "key": "post_sec", "type": "number", "default": 10, "min": 1, "tip": "Keep recording for N seconds after last detected motion." },
      { "key": "min_hits", "type": "number", "default": 1, "min": 1, "tip": "Consecutive motion hits required before entering active state." }
    ]
  }
]
```

### 4) Module config API

- `GET /api/record/config`
- `POST /api/record/config`

Body:

```json
{
  "dir": "./records",
  "retention": 7
}
```

### 5) File management

- `GET /api/record?path={rel_dir}`: list directory items.
- `GET /api/record/file?path={rel_file}`: stream file.
- `GET /api/record/file?path={rel_file}&download=1`: download file.
- `DELETE /api/record/file?path={rel_file}`: delete file.

Security:

- Paths are constrained under record `dir`.
- Traversal outside base dir is rejected.

## Trigger docs entry

To avoid doc drift, trigger behavior and extension guide are maintained only in:

- [`internal/record/trigger/README.md`](./trigger/README.md)
