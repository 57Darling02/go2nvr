# Record Module

`internal/record` is a lightweight NVR recorder layered on the in-tree go2rtc stream core.

## Design

- A per-stream session owns attach/detach, manual and trigger intent, retry state, and the bound `*streams.Stream` pointer.
- Sessions reconcile every second. Reconnecting a producer keeps its recorder; deleting and recreating a stream closes the old recorder before a new one attaches.
- RTP callbacks only clone once and enqueue into a byte-bounded mailbox. One writer goroutine owns the muxer, segment file, flush, sync, and close.
- On queue or global-memory pressure, the recorder drains accepted packets, closes the current readable segment, reports `backpressure`, and retries with `1s, 2s, 5s, 10s, 30s, 60s` backoff.
- Prebuffering requires a recent keyframe and is capped by both time and bytes. A stream without a usable keyframe does not accumulate undecodable history.
- Thumbnail conversion uses a shared latest-only worker pool and a five-second FFmpeg deadline.

## YAML Configuration

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
      trigger_interval: 250
      trigger_params:
        threshold: 14
        post_sec: 10
        min_hits: 1
```

`limits` is deployment-level configuration. It is intentionally absent from the Web UI and `/api/record/config`.

- `memory_mb` is the shared cap for prebuffer, writer queues, and snapshot work.
- `prebuffer_mb` is the per-recorder prebuffer cap.
- `writer_queue_mb` is the per-recorder mailbox cap.
- `snapshot_workers` is in the range `1..4`.
- `memory_mb` must be at least `prebuffer_mb + writer_queue_mb + 2`. Invalid limits are replaced with the defaults and logged.

`dir` defaults to `./records`; `retention` defaults to seven days. Rules are persisted as one transaction, and in-memory configuration changes only after persistence succeeds.

## Storage Layout

New recordings use only this layout:

```text
<record.dir>/streams/<sha256(normalized-source)>/
  metadata.json
  YYYY-MM-DD/
    <unique>.mp4
    <unique>.thumb
```

`metadata.json` records the layout version, source name, and storage ID. Segment names use nanoseconds and exclusive creation, so repeated starts cannot overwrite a file.

Old pre-layout recordings are intentionally not migrated or exposed after upgrade. Retention scans only the `streams/` layout.

## HTTP API

### Recording state and control

- `GET /api/record`
- `GET /api/record?src={stream}`
- `POST /api/record?src={stream}&action=start|stop`

`status` remains compatible with existing clients:

- `recording`: actively writing a segment.
- `idle`: attached and ready, including prebuffer/trigger sessions.
- `stopped`: no recorder is attached.

The state may additionally include:

```json
{
  "phase": "recording",
  "desired_recording": true,
  "last_error": "backpressure",
  "retry_at": "2026-07-19T12:00:01Z",
  "stop_reason": "backpressure",
  "storage_id": "<sha256>"
}
```

Start or stop returns `200` when complete, `202` while attaching, draining, or backing off, `404` for an unknown stream, and `503` when an attached recorder cannot become available.

### Rules and module config

- `GET|POST|DELETE /api/record/rules`
- `GET /api/record/triggers`
- `GET|POST /api/record/config`

Rule fields are `src`, `prebuffer`, `trigger_id`, `trigger_interval`, and `trigger_params`. The module-config API accepts only `dir` and `retention`.

### File management

- `GET /api/record?path=.` lists sources.
- `GET /api/record?path=streams/{storage_id}` lists dates.
- `GET /api/record?path=streams/{storage_id}/{YYYY-MM-DD}` lists segments.
- `GET|DELETE /api/record/file?path=streams/{storage_id}/{YYYY-MM-DD}/{file}` serves or deletes a media file.

List entries provide stable `path` and display `name`. The API rejects traversal, legacy layouts, symlinks, metadata files, and non-recording extensions. `.mp4`, `.mjpeg`, and their `.thumb` companions are the only accepted files.
