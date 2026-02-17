# Record

The `record` module completes go2rtc’s recording capabilities. It turns the project into a fully-fledged, yet lightweight, NVR solution.

It is designed with a **dual-mode philosophy**:

1. **Universal Manual Recording**: Any stream in go2rtc can be recorded instantly via API, **without any prior configuration**.
2. **Automated Strategies**: You can define persistence rules (like "Motion Detection" or "Always On") in the config file or via API for specific streams.

---

## 1. Core Concepts

Understanding the three states of a recorder:

| State | Description | API Status |
| --- | --- | --- |
| **Stopped** | The recorder is inactive. No configuration exists, or the stream is not running. | `stopped` |
| **Idle** | **(Auto Mode Only)** The system is "Armed". It is buffering data and analyzing motion, waiting for a trigger. | `idle` |
| **Recording** | Data is actively being written to the disk (triggered manually or by automation). | `recording` |

---

## 2. API: Control & Status

Monitor and control recordings in real-time.

### List Active Recordings

**GET** `/api/record`

Returns a JSON list of all available streams and their recording status.

```json
[
  {
    "name": "camera1",
    "status": "recording",
    "file": "records/camera1/2023-10-27/12-00-00.000.mp4",
    "duration": "10m30s"
  },
  {
    "name": "office_cam",
    "status": "idle",
    "mode": "motion"
  },
  {
    "name": "doorbell",
    "status": "stopped"
  }
]
```

### Check Stream Status

**GET** `/api/record?src={stream_name}`

Get the detailed state of a recorder. Useful for debugging (e.g., seeing if a "motion" recorder is currently `idle` or `recording`, or checking the status of a stream with no automation config).

```json
{
  "status": "recording",    // stopped | idle | recording
  "mode": "motion",         // Configured auto-strategy (if any)
  "manual": true,           // Is it currently forced on by API?
  "auto_active": false,     // Is it currently triggered by automation?
  "file": "path/to/file",   // Current file path (if recording)
  "duration": "10s"         // Current recording duration
}
```

### Manual Control

**POST** `/api/record?src={stream_name}&action=start|stop`

* **Start**: Instantly starts recording. If the stream has no config, it defaults to manual mode.
* **Stop**: Stops the recording. If the stream was in "Auto" mode, it reverts to `idle` (armed) state.

```bash
# Example: Start recording "camera_living_room"
curl -X POST "http://localhost:1984/api/record?src=camera_living_room&action=start"
```

---

## 3. API: Automation Rules

Dynamically manage recording rules without restarting. Changes are **automatically saved** to `go2rtc.yaml`.

### Get Rules

**GET** `/api/record/rules`

* **Default**: Returns a JSON list of all configured automation rules.
* **?src={stream_name}**: Returns the specific rule for the given stream.

```json
{
  "src": "camera2",
  "mode": "motion",
  "segment": 300,
  "prebuffer": 8,
  "post": 30,
  "threshold": 1500
}
```

### Add / Update Rule

**POST** `/api/record/rules`

Adds a new rule or updates an existing one (matched by `src`).

**Body (JSON):**
```json
{
  "src": "camera2",
  "mode": "motion",       // "always" or "motion"
  "segment": 300,         // Split file every 300s
  "prebuffer": 8,         // Pre-record 8s before event
  "post": 30,             // Record 30s after event
  "threshold": 1500       // Motion sensitivity (optional)
}
```

### Delete Rule

**DELETE** `/api/record/rules?src={stream_name}`

Removes the rule for a stream and stops any automation associated with it.

---

## 4. API: Global Configuration

Manage global recording settings (directory, retention) dynamically.

### Get Configuration

**GET** `/api/record/config`

Returns current global settings.

```json
{
  "dir": "./records",
  "retention": 7
}
```

### Update Configuration

**POST** `/api/record/config`

Updates global settings. Changes are **automatically saved** to `go2rtc.yaml`.

* **dir**: Path to recording directory (will be created if not exists).
* **retention**: Retention period in days (0 to disable).

```json
{
  "dir": "/mnt/storage/cctv",
  "retention": 30
}
```

---

## 5. API: File Management

Browse, download, and manage recorded footage.

### Browse Files (JSON)

**GET** `/api/record?path={folder_path}`

Returns a JSON list of files and subdirectories. Use this to build a file browser UI.

* **path**: Relative path from the recording root (default: root).

```json
[
  {"name": "camera1", "is_file": false},
  {"name": "event.mp4", "is_file": true, "size": 10240, "mod_time": 1698390000}
]
```

### Stream / Download File

**GET** `/api/record/file?path={file_path}`

* **Default**: Streams the file content (supports HTTP Range requests for seeking).
* **&download=1**: Sets `Content-Disposition` header to force download.

### Delete File

**DELETE** `/api/record/file?path={file_path}`

Permanently deletes the specified file.

---

## 5. Configuration (YAML)

You can also configure automation manually in `go2rtc.yaml`.
*Note: The API methods above (Section 3) are the recommended way to manage this programmatically.*

```yaml
record:
  dir: ./records       # Storage root
  retention: 7         # Delete files older than 7 days (0 to disable)
  
  rules:             
    - src: camera1
      mode: always     # 24/7 Recording
      segment: 600     # Split files every 10 minutes

    - src: camera2
      mode: motion     # Record only when motion is detected
      segment: 300
      prebuffer: 8     # Keep 8s of video before motion happens
      post: 30         # Keep recording 30s after motion stops
      threshold: 5000  # Sensitivity (lower = more sensitive)
```

### Directory Structure

Files are organized automatically:
`./records/{stream_name}/{YYYY-MM-DD}/{HH-MM-SS.mmm}.mp4`

---

## 6. Features & Architecture

This module is built for production-grade reliability and performance.

### 🚀 High-Performance I/O

* **Buffered Writing**: Uses a 64KB `bufio` buffer to merge tiny RTP packets into large disk blocks. This drastically reduces syscalls (IOPS) and CPU usage.
* **Zero-Copy Logic**: Heavy operations are minimized. Data is only cloned when necessary (e.g., for the pre-record buffer).

### 🔄 Seamless Segmentation

* **Gapless Rotation**: The file rotation logic is optimized: New files are pre-created, and the pointer is swapped under a lock.
* **Async Closing**: Old files are flushed and closed in a background goroutine, ensuring the main RTP processing loop never blocks or drops frames during file switches.

### 🧠 Smart Motion Detection

* **Pre-Recording**: Uses an in-memory ring buffer to save video *before* the trigger event, ensuring you never miss the start of the action.
* **Safety Timeout**: FFmpeg processes used for motion analysis are protected by a strict `context` timeout to prevent zombie processes.

### 🧹 Auto Maintenance

* **Retention Policy**: A background task runs hourly to clean up old recordings based on your `retention` setting, making the system maintenance-free.
