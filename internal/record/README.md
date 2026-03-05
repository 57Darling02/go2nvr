# Record 模块

`record` 模块为 go2rtc 提供了灵活的录制功能，使其能够作为一个轻量级的 NVR (网络视频录像机) 运行。

在重构后，该模块采用了**极致解耦**的设计理念：将录制执行、配置管理与外部触发逻辑完全分离。

---

## 1. 核心设计理念

- **手动触发为主**: 录制的开始与结束主要通过 API 显式调用。
- **预录缓存 (Pre-buffering)**: 即使在未录制时，系统也会根据配置维护一个内存环形缓冲，确保手动启动录制时能包含触发前的数据。
- **外部驱动**: 运动检测、定时计划等复杂逻辑不再内置，而是通过独立的进程或逻辑调用 API 来驱动本模块。

---

## 2. API 接口说明

### 录制控制与状态

**GET** `/api/record`
返回所有流的实时录制状态。

```json
[
  {
    "name": "camera1",
    "status": "recording",
    "file": "records/camera1/2026-03-05/12-00-00.mp4",
    "duration": "10m30s"
  },
  {
    "name": "office_cam",
    "status": "idle",
    "prebuffer": 10
  }
]
```

**POST** `/api/record?src={stream_name}&action=start|stop`
- **start**: 立即启动录制。如果存在预录缓存，会先将缓存数据回灌入文件。
- **stop**: 停止录制并关闭文件。

---

### 自动化规则管理

**GET** `/api/record/rules`
获取当前定义的录制规则列表。

**POST** `/api/record/rules`
添加或更新规则。规则仅定义流的默认预录时长，不负责自动开启录制。
**Body (JSON):**
```json
{
  "src": "camera1",
  "prebuffer": 10
}
```

**DELETE** `/api/record/rules?src={stream_name}`
删除指定流的规则。

---

### 全局配置

**GET** `/api/record/config`
**POST** `/api/record/config`
管理全局录制路径和文件保留天数。
```json
{
  "dir": "./records",
  "retention": 7
}
```

---

### 文件管理

**GET** `/api/record?path={folder_path}`
浏览录制目录下的文件列表。

**GET** `/api/record/file?path={file_path}`
- 直接访问：流式播放/预览。
- `&download=1`: 强制下载。

**DELETE** `/api/record/file?path={file_path}`
永久删除录制文件。

---

## 3. 配置文件说明 (YAML)

你可以在 `go2rtc.yaml` 中静态配置：

```yaml
record:
  dir: ./records       # 录制根目录
  retention: 7         # 自动清理 7 天前的录像 (0 为禁用)
  
  rules:             
    - src: camera1
      prebuffer: 10    # 始终为 camera1 维护 10 秒预录缓存
```

---

## 4. 架构与性能特性

### 🚀 高性能 I/O
- **缓冲写入**: 使用 64KB `bufio` 缓冲，减少磁盘寻址与系统调用。
- **异步关闭**: 文件关闭操作在独立协程执行，避免阻塞实时 RTP 转发路径。

### 🔄 预录逻辑 (Pre-buffer)
- 每个 Recorder 维护一个基于时间戳的内存队列。
- 当收到 `start` 指令时，Recorder 会定位到缓存中最近的一个关键帧 (Keyframe) 开始回灌数据，确保生成的 MP4 文件能够正常解码。

### 🛡️ 路径安全
- 所有文件操作 API 均内置了路径穿越 (Path Traversal) 检查，确保无法访问录制目录以外的系统文件。

### 🧹 自动维护
- 后台任务每小时运行一次，根据 `retention` 设定自动删除过期日期的目录，实现无人值守运行。
