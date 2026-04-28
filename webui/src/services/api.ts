export interface StreamProducer {
  url?: string
  [key: string]: any
}

export interface StreamConsumer {
  [key: string]: any
}

export interface Stream {
  producers?: StreamProducer[]
  consumers?: StreamConsumer[]
  [key: string]: any
}

export interface StreamInfo {
  [key: string]: Stream
}

export interface Recording {
  name: string
  is_file?: boolean
  size?: number
  mod_time?: number
  // Legacy/Optional fields
  dur?: number
  time?: string
  url?: string
  type?: 'file' | 'dir'
  [key: string]: any
}

export interface ServerInfo {
  config_path?: string
  host?: string
  rtsp?: {
    listen?: string
    default_query?: string
  }
  version?: string
  [key: string]: any
}

const API_BASE = ''

async function getErrorMessage(res: Response, fallback: string): Promise<string> {
  const status = `${res.status}${res.statusText ? ` ${res.statusText}` : ''}`
  let detail = ''
  try {
    const contentType = res.headers.get('content-type') || ''
    if (contentType.includes('application/json')) {
      const data = await res.json()
      detail = String(data?.message || data?.error || data?.detail || '').trim()
      if (!detail && data !== undefined && data !== null) {
        detail = JSON.stringify(data)
      }
    } else {
      detail = (await res.text()).trim()
    }
  } catch (_) {
    detail = ''
  }
  if (detail) {
    return `${fallback} (${status}): ${detail}`
  }
  return `${fallback} (${status})`
}

// --- Streams ---

export async function getStreams(): Promise<string[]> {
  try {
    const res = await fetch(`${API_BASE}/api/streams`)
    if (!res.ok) throw new Error(`Failed to fetch streams: ${res.statusText}`)
    const data = await res.json()
    return Object.keys(data || {}).sort()
  } catch (error) {
    console.error('Error fetching streams:', error)
    return []
  }
}

export async function getAllStreamsInfo(): Promise<StreamInfo> {
  try {
    const res = await fetch(`${API_BASE}/api/streams`)
    if (!res.ok) throw new Error(`Failed to fetch streams info: ${res.statusText}`)
    return await res.json()
  } catch (error) {
    console.error('Error fetching streams info:', error)
    return {}
  }
}

export async function getStreamInfo(src: string): Promise<Stream | null> {
  try {
    const res = await fetch(`${API_BASE}/api/streams?src=${encodeURIComponent(src)}`)
    if (!res.ok) return null
    return await res.json()
  } catch (error) {
    console.error(`Error fetching stream info for ${src}:`, error)
    return null
  }
}

export async function addStream(name: string, src: string): Promise<void> {
  const params = new URLSearchParams({ name, src })
  const res = await fetch(`${API_BASE}/api/streams?${params.toString()}`, {
    method: 'PUT',
  })
  if (!res.ok) throw new Error(await getErrorMessage(res, 'Failed to add stream'))
}

export async function updateStream(name: string, src: string): Promise<void> {
  const params = new URLSearchParams({ name, src })
  const res = await fetch(`${API_BASE}/api/streams?${params.toString()}`, {
    method: 'PATCH',
  })
  if (!res.ok) throw new Error(`Failed to update stream: ${res.statusText}`)
}

export async function deleteStream(src: string): Promise<void> {
  const params = new URLSearchParams({ src })
  const res = await fetch(`${API_BASE}/api/streams?${params.toString()}`, {
    method: 'DELETE',
  })
  if (!res.ok) throw new Error(`Failed to delete stream: ${res.statusText}`)
}

export async function sendStream(src: string, dst: string): Promise<void> {
  const params = new URLSearchParams({ src, dst })
  const res = await fetch(`${API_BASE}/api/streams?${params.toString()}`, {
    method: 'POST',
  })
  if (!res.ok) throw new Error(`Failed to send stream: ${res.statusText}`)
}

export async function getStreamsGraph(src?: string): Promise<string> {
  const params = src ? `?src=${encodeURIComponent(src)}` : ''
  const res = await fetch(`${API_BASE}/api/streams.dot${params}`)
  if (!res.ok) throw new Error(`Failed to fetch streams graph: ${res.statusText}`)
  return await res.text()
}

// --- Preload ---

export async function getPreloads(): Promise<any> {
  const res = await fetch(`${API_BASE}/api/preload`)
  if (!res.ok) throw new Error(`Failed to fetch preloads: ${res.statusText}`)
  return await res.json()
}

export async function addPreload(src: string): Promise<void> {
  const params = new URLSearchParams({ src })
  const res = await fetch(`${API_BASE}/api/preload?${params.toString()}`, {
    method: 'PUT',
  })
  if (!res.ok) throw new Error(`Failed to add preload: ${res.statusText}`)
}

export async function deletePreload(src: string): Promise<void> {
  const params = new URLSearchParams({ src })
  const res = await fetch(`${API_BASE}/api/preload?${params.toString()}`, {
    method: 'DELETE',
  })
  if (!res.ok) throw new Error(`Failed to delete preload: ${res.statusText}`)
}

// --- Consumption / Formats ---

export function getStreamUrl(
  src: string,
  protocol: 'ws' | 'hls' | 'mp4' | 'mjpeg' | 'jpeg' | 'info',
  options: any = {},
): string {
  let endpoint = ''
  switch (protocol) {
    case 'ws':
      endpoint = '/api/ws'
      break
    case 'hls':
      endpoint = '/api/stream.m3u8'
      break
    case 'mp4':
      endpoint = '/api/stream.mp4'
      break
    case 'mjpeg':
      endpoint = '/api/stream.mjpeg'
      break
    case 'jpeg':
      endpoint = '/api/frame.jpeg'
      break
    case 'info':
      endpoint = '/api/streams'
      break
  }
  const params = new URLSearchParams({ src, ...options })
  return `${API_BASE}${endpoint}?${params.toString()}`
}

export async function getSnapshot(src: string): Promise<Blob> {
  const url = getStreamUrl(src, 'jpeg')
  const res = await fetch(url)
  if (!res.ok) throw new Error(`Failed to get snapshot: ${res.statusText}`)
  return await res.blob()
}

// --- WebRTC ---

export async function getWebRTC(src: string, sdpOffer?: string): Promise<any> {
  const params = new URLSearchParams({ src })
  const res = await fetch(`${API_BASE}/api/webrtc?${params.toString()}`, {
    method: 'POST',
    headers: {
      'Content-Type': sdpOffer ? 'application/sdp' : 'application/json', // Or assume JSON if no offer
    },
    body: sdpOffer || undefined, // If no offer, maybe we just want to init? Spec says required.
  })
  if (!res.ok) throw new Error(`Failed to get WebRTC: ${res.statusText}`)
  const contentType = res.headers.get('content-type')
  if (contentType && contentType.includes('application/json')) {
    return await res.json()
  } else {
    return await res.text()
  }
}

// --- FFmpeg ---

export async function playFFmpeg(
  dst: string,
  options: { file?: string; live?: string; text?: string; voice?: string },
): Promise<void> {
  const params = new URLSearchParams({ dst, ...options })
  const res = await fetch(`${API_BASE}/api/ffmpeg?${params.toString()}`, {
    method: 'POST',
  })
  if (!res.ok) throw new Error(`Failed to play FFmpeg: ${res.statusText}`)
}

// --- TTS Push ---

export type TtsPushTargetType = 'ipwebcam' | 'wss' | 'backchannel' | 'stream'

export interface TtsPushPayload {
  text: string
  target_type?: TtsPushTargetType
  target?: string
  url?: string
  dst?: string
  format?: 'wav'
  voice?: string
  rate?: string
  pitch?: string
  volume?: string
  proxy?: string
  connect_timeout?: number
  receive_timeout?: number
  sample_rate?: number
  chunk_ms?: number
  realtime?: boolean
  insecure_tls?: boolean
}

export async function pushTts(payload: TtsPushPayload): Promise<any> {
  const res = await fetch(`${API_BASE}/api/ttspush/push`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) throw new Error(await getErrorMessage(res, 'Failed to push TTS'))
  const contentType = res.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    return await res.json()
  }
  const text = (await res.text()).trim()
  return text || undefined
}

// --- Discovery ---

export async function discover(
  type: 'onvif' | 'hass' | 'homekit' | 'roborock' | 'tuya',
  options: any = {},
): Promise<any> {
  const params = new URLSearchParams(options)
  const res = await fetch(`${API_BASE}/api/${type}?${params.toString()}`)
  if (!res.ok) throw new Error(await getErrorMessage(res, `Failed to discover ${type}`))
  return await res.json()
}

export interface RecordingRule {
  src: string
  prebuffer?: number
  trigger_id?: number
  trigger_threshold?: number
  trigger_post?: number
  trigger_interval?: number
  trigger_params?: Record<string, any>
}

export interface StreamStatusItem {
  name: string
  status: 'stopped' | 'idle' | 'recording'
  prebuffer?: number
  trigger_id?: number
  trigger_key?: string
  trigger_name?: string
  file?: string
  duration?: string
}

export interface RecordingStatus {
  status: 'stopped' | 'idle' | 'recording'
  prebuffer?: number
  trigger_id?: number
  trigger_key?: string
  trigger_name?: string
  file?: string
  duration?: string
}

export interface TriggerMeta {
  id: number
  key: string
  name: string
  params?: TriggerParamMeta[]
}

export interface TriggerParamMeta {
  key: string
  type?: string
  name?: string
  default?: any
  min?: number
  max?: number
  step?: number
  tip?: string
  required?: boolean
  options?: Array<{ label?: string; value: any }>
}

// --- Recording (Existing) ---

export async function getRecordings(path = '.'): Promise<Recording[]> {
  try {
    const res = await fetch(`${API_BASE}/api/record?path=${encodeURIComponent(path)}`)
    if (!res.ok) throw new Error(`Failed to fetch recordings: ${res.statusText}`)
    const data = await res.json()
    if (!Array.isArray(data)) return []
    return data
  } catch (error) {
    console.error('Error fetching recordings:', error)
    return []
  }
}

/**
 * Get list of all available streams and their recording status.
 */
export async function getAllRecordingsStatus(): Promise<StreamStatusItem[]> {
  const res = await fetch(`${API_BASE}/api/record`)
  if (!res.ok) throw new Error(`Failed to get all recordings status: ${res.statusText}`)
  return await res.json()
}

/**
 * Get detailed status of a specific stream recorder.
 */
export async function getStreamRecordingStatus(src: string): Promise<RecordingStatus> {
  const res = await fetch(`${API_BASE}/api/record?src=${encodeURIComponent(src)}`)
  if (!res.ok) throw new Error(`Failed to get stream recording status: ${res.statusText}`)
  return await res.json()
}

/**
 * Legacy support: Get recording status for all streams (or specific if src provided).
 * Returns StreamStatusItem[] if src is undefined.
 * Returns RecordingStatus if src is provided.
 */
export async function getRecordingStatus(src?: string): Promise<any> {
  if (src) {
    return getStreamRecordingStatus(src)
  }
  return getAllRecordingsStatus()
}

export async function startRecording(src: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/record?src=${encodeURIComponent(src)}&action=start`, {
    method: 'POST',
  })
  if (!res.ok) throw new Error(`Failed to start recording: ${res.statusText}`)
}

export async function stopRecording(src: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/record?src=${encodeURIComponent(src)}&action=stop`, {
    method: 'POST',
  })
  if (!res.ok) throw new Error(`Failed to stop recording: ${res.statusText}`)
}

export interface RecordingConfig {
  dir: string
  retention: number
}

// --- Recording Rules ---

export async function getRecordingRules(src?: string): Promise<RecordingRule[] | RecordingRule> {
  const url = src
    ? `${API_BASE}/api/record/rules?src=${encodeURIComponent(src)}`
    : `${API_BASE}/api/record/rules`
  const res = await fetch(url)
  if (!res.ok) throw new Error(`Failed to get recording rules: ${res.statusText}`)
  return await res.json()
}

export async function addRecordingRule(rule: RecordingRule): Promise<void> {
  const res = await fetch(`${API_BASE}/api/record/rules`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(rule),
  })
  if (!res.ok) throw new Error(await getErrorMessage(res, 'Failed to add/update recording rule'))
}

export async function deleteRecordingRule(src: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/record/rules?src=${encodeURIComponent(src)}`, {
    method: 'DELETE',
  })
  if (!res.ok) throw new Error(await getErrorMessage(res, 'Failed to delete recording rule'))
}

export async function getRecordingTriggers(): Promise<TriggerMeta[]> {
  const res = await fetch(`${API_BASE}/api/record/triggers`)
  if (!res.ok) throw new Error(`Failed to get recording triggers: ${res.statusText}`)
  return await res.json()
}

// --- Recording Global Config ---

export async function getRecordingConfig(): Promise<RecordingConfig> {
  const res = await fetch(`${API_BASE}/api/record/config`)
  if (!res.ok) throw new Error(`Failed to get recording config: ${res.statusText}`)
  return await res.json()
}

export async function updateRecordingConfig(config: Partial<RecordingConfig>): Promise<void> {
  const res = await fetch(`${API_BASE}/api/record/config`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  })
  if (!res.ok) throw new Error(await getErrorMessage(res, 'Failed to update recording config'))
}

export function getRecordingFileUrl(path: string, download = false): string {
  const baseUrl = `${API_BASE}/api/record/file`
  const params = new URLSearchParams({ path })
  if (download) params.append('download', '1')
  return `${baseUrl}?${params.toString()}`
}

export async function deleteRecordingFile(path: string): Promise<void> {
  const url = getRecordingFileUrl(path)
  const res = await fetch(url, { method: 'DELETE' })
  if (!res.ok) throw new Error(await getErrorMessage(res, 'Failed to delete recording file'))
}

// --- System ---

export async function getServerInfo(): Promise<ServerInfo> {
  try {
    const res = await fetch(`${API_BASE}/api`)
    if (!res.ok) throw new Error(`Failed to get server info: ${res.statusText}`)
    return await res.json()
  } catch (error) {
    console.error('Error fetching server info:', error)
    return {}
  }
}

export async function getConfig(): Promise<string> {
  const res = await fetch(`${API_BASE}/api/config`)
  if (!res.ok) throw new Error(`Failed to get config: ${res.statusText}`)
  return await res.text()
}

export async function saveConfig(yaml: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/config`, {
    method: 'POST',
    body: yaml,
  })
  if (!res.ok) throw new Error(`Failed to save config: ${res.statusText}`)
}

export async function getLog(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/api/log`)
  if (!res.ok) throw new Error(`Failed to fetch logs: ${res.statusText}`)
  const text = await res.text()
  return text
    .trim()
    .split('\n')
    .map((line) => {
      try {
        return JSON.parse(line)
      } catch (e) {
        return { message: line }
      }
    })
}

export async function clearLog(): Promise<void> {
  const res = await fetch(`${API_BASE}/api/log`, { method: 'DELETE' })
  if (!res.ok) throw new Error(`Failed to clear logs: ${res.statusText}`)
}

export async function getStack(): Promise<string> {
  const res = await fetch(`${API_BASE}/api/stack`)
  if (!res.ok) throw new Error(`Failed to fetch stack trace: ${res.statusText}`)
  return await res.text()
}

export async function restartServer(): Promise<void> {
  const res = await fetch(`${API_BASE}/api/restart`, { method: 'POST' })
  if (!res.ok) throw new Error(`Failed to restart server: ${res.statusText}`)
}

export async function exitServer(code: number = 0): Promise<void> {
  const params = new URLSearchParams({ code: code.toString() })
  const res = await fetch(`${API_BASE}/api/exit?${params.toString()}`, { method: 'POST' })
  if (!res.ok) throw new Error(`Failed to stop server: ${res.statusText}`)
}

export async function getSchemes(): Promise<string[]> {
  const res = await fetch(`${API_BASE}/api/schemes`)
  if (!res.ok) throw new Error(`Failed to fetch schemes: ${res.statusText}`)
  return await res.json()
}
