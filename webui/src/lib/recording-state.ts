export type StreamRuntimeStatus = 'stopped' | 'idle' | 'recording'

export type StreamRuntimeState = {
  status: StreamRuntimeStatus
  triggerId: number
  triggerKey: string
  triggerName: string
}

export type StreamRuntimeStateMap = Record<string, StreamRuntimeState>

function normalizeStatus(value: unknown): StreamRuntimeStatus {
  if (value === 'recording' || value === 'idle' || value === 'stopped') {
    return value
  }
  return 'stopped'
}

export function parseRecordingRuntimeState(raw: unknown): StreamRuntimeStateMap {
  const next: StreamRuntimeStateMap = {}

  if (Array.isArray(raw)) {
    for (const item of raw) {
      if (item && typeof item === 'object' && 'name' in item && 'status' in item) {
        const row = item as Record<string, unknown>
        const name = String(row.name || '').trim()
        if (!name) continue
        next[name] = {
          status: normalizeStatus(row.status),
          triggerId:
            Number(row.trigger_id ?? row.TriggerId ?? row.trigger ?? row.Trigger ?? 0) || 0,
          triggerKey: String(
            row.trigger_key ?? row.TriggerKey ?? row.trigger_type ?? row.TriggerType ?? '',
          ),
          triggerName: String(row.trigger_name ?? row.TriggerName ?? ''),
        }
        continue
      }

      if (typeof item === 'string') {
        next[item] = {
          status: 'recording',
          triggerId: 0,
          triggerKey: '',
          triggerName: '',
        }
      }
    }
    return next
  }

  if (raw && typeof raw === 'object') {
    for (const [stream, status] of Object.entries(raw)) {
      next[stream] = {
        status: status === 'recording' ? 'recording' : 'stopped',
        triggerId: 0,
        triggerKey: '',
        triggerName: '',
      }
    }
  }

  return next
}

export function getStreamStatus(state: StreamRuntimeStateMap, stream: string): StreamRuntimeStatus {
  return state[stream]?.status || 'stopped'
}

export function isTriggerEnabled(state: StreamRuntimeStateMap, stream: string): boolean {
  return (state[stream]?.triggerId || 0) > 0
}

export function getTriggerType(state: StreamRuntimeStateMap, stream: string): string {
  const streamState = state[stream]
  if (!streamState) return 'trigger'
  return streamState.triggerName || streamState.triggerKey || 'trigger'
}

export function getStatusClass(state: StreamRuntimeStateMap, stream: string): string {
  const status = getStreamStatus(state, stream)
  if (status === 'recording') return 'bg-red-500 animate-pulse shadow-[0_0_8px_rgba(239,68,68,0.6)]'
  if (status === 'idle') return 'bg-yellow-500 shadow-[0_0_8px_rgba(234,179,8,0.4)]'
  return 'bg-gray-600 group-hover:bg-gray-500'
}

export function getToggleTitle(state: StreamRuntimeStateMap, stream: string): string {
  const status = getStreamStatus(state, stream)
  if (status === 'recording') return 'Stop Recording'
  if (status === 'idle' && isTriggerEnabled(state, stream)) return 'Start Recording (Trigger Armed)'
  if (status === 'idle') return 'Start Recording (Pre-buffer Ready)'
  return 'Start Recording'
}
