# Record 模块

`record` 模块为 go2rtc 提供录制能力，并且保持“录制执行”和“触发检测”解耦：  
- `recorder` 负责预录、写盘、状态  
- `trigger` 子模块负责检测与触发 `Start/Stop`

---

## 1. 核心设计理念

- **录制幂等化**：重复 `start` 不会重复开启录制文件，重复 `stop` 也不会破坏状态。
- **预录优先**：始终维护预录缓存，触发后可回灌关键帧前后的内容。
- **触发解耦**：检测算法不放进录制主链路，按规则独立协程运行。
- **可插拔触发器**：`trigger` 子模块可注册多种检测器，默认内置 `motion_diff`。

---

## 2. API 接口说明

### 录制控制与状态

**GET** `/api/record`  
返回所有流录制状态（包含 trigger 信息）。

```json
[
  {
    "name": "camera1",
    "status": "recording",
    "file": "records/camera1/2026-03-05/12-00-00.mp4",
    "duration": "10m30s",
    "prebuffer": 10,
    "trigger_id": 1,
    "trigger_key": "motion_diff",
    "trigger_name": "Motion Diff"
  }
]
```

**POST** `/api/record?src={stream_name}&action=start|stop`
- **start**: 立即开始录制；若已有录制则忽略重复启动。
- **stop**: 停止录制并关闭文件。

### 自动化规则管理

**GET** `/api/record/rules`  
获取全部规则；可加 `?src=` 查询单条规则。

**POST** `/api/record/rules`  
新增或更新规则。支持录制参数 + trigger 参数。

```json
{
  "src": "camera1",
  "prebuffer": 10,
  "trigger_id": 1,
  "trigger_threshold": 14,
  "trigger_post": 10,
  "trigger_interval": 250
}
```

- `trigger_id`: 主字段，`>0` 启用触发器，`0` 或省略表示禁用
- `trigger_threshold`: 检测阈值（检测器自行解释）
- `trigger_post`: 触发后延时停止秒数
- `trigger_interval`: 检测周期，单位毫秒

**DELETE** `/api/record/rules?src={stream_name}`  
删除规则并停止该流 trigger 协程。

### 触发器元数据

**GET** `/api/record/triggers`  
返回所有可用触发器的 ID、键名和显示名（供前端下拉框使用）。

```json
[
  { "id": 1, "key": "motion_diff", "name": "Motion Diff" }
]
```

### 全局配置

**GET** `/api/record/config` / **POST** `/api/record/config`  
管理全局录制目录和保留天数。

### 文件管理

**GET** `/api/record?path={folder_path}`：浏览录制目录  
**GET** `/api/record/file?path={file_path}`：预览/下载  
**DELETE** `/api/record/file?path={file_path}`：删除文件

---

## 3. 配置示例 (YAML)

```yaml
record:
  dir: ./records
  retention: 7
  rules:
    - src: camera1
      prebuffer: 10
      trigger_id: 1
      trigger_threshold: 14
      trigger_post: 10
      trigger_interval: 250
```

---

## 4. 动作检测完整生命周期

### 启动阶段

1. `record.Init()` 加载规则并初始化 trigger manager。  
2. 对每条规则执行 `startTriggerForRule(rule)`。  
3. 若规则触发启用（`trigger_id > 0`），创建对应 worker 协程。

### 运行阶段

1. worker 按 `trigger_interval` 定时轮询。  
2. 通过 recorder 暴露接口读取“最近关键帧快照”（内存复制，避免共享写冲突）。  
3. trigger 入口统一生成标准 `Frame`：包含固定尺寸灰度帧 `Frame.Gray`（当前为 `64x36`）与原始快照 `Frame.JPEG`。  
4. 仅当拿到新时间戳帧时做检测，未更新帧直接跳过。  
5. 检测命中会累积 `moveCount`，达到阈值后触发 `record.Start(src)`，并刷新 `lastMotion`。  
6. 若外部手动停止，worker 会按实际录制状态自动同步内部 `active`。  
7. 长时间未命中且超过 `trigger_post` 时触发 `record.Stop(src)`。

### 变更与退出阶段

- 规则 POST：先替换同源旧 worker，再按新参数启动。  
- 规则 DELETE：停止 worker 并关闭录制。  
- 服务退出：worker 收到 stop 信号后安全退出。

---

## 5. Trigger 子模块与 Collaborator Guide

目录：

```text
internal/record/trigger/
  trigger.go            # 子模块入口（注册、manager、worker生命周期）
  motion_diff.go        # 默认检测器
```

### 协作者开发新触发器（只关注算法）

1. 新建一个 `xxx_detector.go`。  
2. 定义检测器结构并实现 `Detect(prev, cur Frame) bool`。  
3. 在 `init()` 中注册：

```go
func init() {
    Register(2, "my_detector", "My Detector", NewMyDetector)
}
```

4. 规则中配置：

```yaml
trigger_id: 2
```

`trigger` 入口会先完成统一输入标准化，`Frame` 同时提供两层数据：  
- `Frame.Gray` + `Frame.Width/Height`：固定尺寸灰度帧（轻量快速，适合 motion diff）。  
- `Frame.JPEG` + `Frame.JPEGWidth/JPEGHeight`：原始快照（信息完整，适合人脸/车辆等 AI 模型）。  

因此协作者无需关心输入流是 H264、H265 还是 JPEG，可以按算法复杂度自由选择“快路径”或“精确路径”。

---

## 6. 架构与性能特性

- **高性能写盘**：64KB 缓冲写入，减少系统调用。  
- **预录回灌**：启动录制时从关键帧开始写入，保证文件可解码。  
- **轻量检测**：复用 recorder 最近关键帧，不重复拉流。  
- **快照复用**：trigger 与录像缩略图优先复用同一份最近快照，减少重复转码。  
- **路径安全**：文件接口带路径穿越校验。  
- **自动维护**：按 `retention` 每小时清理过期目录。

---

## 7. 模块评估（简洁性 / 性能 / 长稳运行）

### 简洁性

- `record.go` 负责入口、API、实例管理。  
- `recorder.go` 负责写盘和预录缓存。  
- `trigger/` 负责算法与触发状态机。  
- 目前分层清晰，职责边界明确，便于后续继续扩展不同 trigger 类型。

### 性能与效率（当前实现）

- 录制链路本身（预录缓冲 + 写盘）较轻量。  
- 触发链路采用“关键帧快照 -> 统一灰度帧 -> 差分判定”。  
- 快照在 `Recorder.writeRTP()` 中生成，trigger 只处理统一的 `Frame`，检测器无需关心编码细节。  
- 缩略图保存优先复用同一份最近快照，无快照时再走回退抓帧路径。  
- 关键点：关键帧到 JPEG 的转换仍在 `Recorder.writeRTP()` 的互斥锁内，且依赖 ffmpeg 进程调用。  
- 在关键帧频率高、路数多时，这段逻辑仍可能成为 CPU/延迟热点。

### 长时间运行的内存风险

当前代码未发现明显“无上限增长”的内存泄漏路径，但有以下需要关注的点：

- **预录缓存**：按 `prebuffer` 时间窗口保留，`pruneBuffer` 会持续裁剪，通常不会无限增长。  
- **关键帧缓存**：`keyBuf` 有 2MB 上限，`lastKey` 只保留最近一帧 JPEG。  
- **trigger worker**：按规则一流一协程，规则删除会停止 worker 并从 map 移除。  
- **风险不在泄漏，而在资源抖动**：若 ffmpeg 转码频繁，可能出现进程开销大、锁竞争明显、触发延迟增加。

### 建议（生产可选）

- 将“关键帧转 JPEG”从 `writeRTP()` 主锁路径移出，改为独立快照协程按需转换。  
- 增加 trigger 级别的最小触发间隔与冷却时间，减少反复 start/stop 抖动。  
- 对多路场景建议从 250ms 调整为 300~500ms，优先保证系统稳定。  
- 需要调阈值时建议开启 `log.level: debug`，结合 `motion_diff detection score` 观察静态/动态区间。
