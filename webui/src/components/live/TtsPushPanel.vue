<template>
  <section class="min-w-0 rounded-md border border-gray-800 bg-gray-900/60 p-3">
    <div class="mb-3 flex items-center justify-between gap-2">
      <div>
        <h3 class="text-sm font-semibold text-white">TTS Push</h3>
        <p class="text-xs text-gray-400">{{ modeSummary }}</p>
      </div>
    </div>

    <div class="mb-3 min-h-[54px]">
      <Alert :class="pushStateClass">
        <AlertDescription>{{ pushStateMessage }}</AlertDescription>
      </Alert>
    </div>

    <form class="min-w-0 space-y-3 overflow-x-hidden" @submit.prevent="submitTts">
      <div class="min-w-0 space-y-2">
        <div class="flex items-center justify-between gap-2">
          <Label for="tts-text" class="text-gray-300">Text</Label>
          <label
            class="inline-flex shrink-0 cursor-pointer items-center gap-2 text-xs text-gray-400"
          >
            <input
              v-model="form.clearTextOnSuccess"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-600 bg-gray-800 text-blue-600 focus:ring-blue-500"
            />
            Clear on success
          </label>
        </div>
        <textarea
          id="tts-text"
          v-model.trim="form.text"
          @keydown="onTextKeydown"
          class="min-h-[96px] w-full resize-y rounded-md border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-white placeholder:text-gray-500 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          placeholder="Type the text to synthesize..."
        />
      </div>

      <button
        type="submit"
        :disabled="!canSubmit"
        class="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-500 disabled:cursor-not-allowed disabled:opacity-60"
      >
        <RefreshCw v-if="pushing" class="h-4 w-4 animate-spin" />
        <span>{{ pushing ? 'Pushing...' : 'Push TTS' }}</span>
      </button>

      <details
        open
        class="min-w-0 rounded-md border border-gray-800 bg-gray-900/50 p-3 text-sm text-gray-300"
      >
        <summary class="cursor-pointer select-none text-xs text-gray-400">Settings</summary>
        <div class="mt-3 min-w-0 space-y-3">
          <div class="grid min-w-0 gap-3 md:grid-cols-2">
            <div class="min-w-0 space-y-2 md:col-span-2">
              <Label for="tts-voice" class="text-gray-300">Voice</Label>
              <Select v-model="form.voice">
                <SelectTrigger
                  id="tts-voice"
                  class="h-9 border-gray-700 bg-gray-900 text-white data-[placeholder]:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent class="border-gray-700 bg-gray-900 text-white">
                  <SelectItem
                    v-for="voice in VOICE_OPTIONS"
                    :key="voice.value"
                    :value="voice.value"
                    class="text-gray-200 focus:bg-gray-800 focus:text-white"
                  >
                    {{ voice.value }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <p class="break-all text-xs text-gray-500">{{ selectedVoiceDescription }}</p>
            </div>
            <div class="min-w-0 space-y-2">
              <Label for="tts-rate" class="text-gray-300">Rate</Label>
              <Input
                id="tts-rate"
                v-model="form.rate"
                type="text"
                placeholder="+0%"
                class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
              />
            </div>
            <div class="min-w-0 space-y-2">
              <Label for="tts-pitch" class="text-gray-300">Pitch</Label>
              <Input
                id="tts-pitch"
                v-model="form.pitch"
                type="text"
                placeholder="+0Hz"
                class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
              />
            </div>
            <div class="min-w-0 space-y-2">
              <Label for="tts-volume" class="text-gray-300">Volume</Label>
              <Input
                id="tts-volume"
                v-model="form.volume"
                type="text"
                placeholder="+0%"
                class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
              />
            </div>
            <div class="min-w-0 space-y-2">
              <Label for="tts-target" class="text-gray-300">Target</Label>
              <Select v-model="form.targetType">
                <SelectTrigger
                  id="tts-target"
                  class="h-9 border-gray-700 bg-gray-900 text-white data-[placeholder]:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent class="border-gray-700 bg-gray-900 text-white">
                  <SelectItem
                    value="backchannel"
                    class="text-gray-200 focus:bg-gray-800 focus:text-white"
                  >
                    Backchannel
                  </SelectItem>
                  <SelectItem
                    value="ipwebcam"
                    class="text-gray-200 focus:bg-gray-800 focus:text-white"
                  >
                    IP Webcam
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div
            v-if="form.targetType === 'backchannel'"
            class="min-w-0 space-y-2 rounded-md border border-gray-800 bg-gray-900/60 p-3"
          >
            <Label for="tts-dst" class="text-gray-300">Destination Stream (dst)</Label>
            <Input
              id="tts-dst"
              v-model="form.dst"
              type="text"
              placeholder="camera1"
              class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
            />
            <label class="inline-flex cursor-pointer items-center gap-2 text-xs text-gray-400">
              <input
                v-model="syncDstWithStream"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-600 bg-gray-800 text-blue-600 focus:ring-blue-500"
              />
              Auto sync dst with selected stream
            </label>
          </div>

          <div
            v-else
            class="min-w-0 space-y-3 rounded-md border border-gray-800 bg-gray-900/60 p-3"
          >
            <div class="grid min-w-0 gap-3 md:grid-cols-2">
              <div class="min-w-0 space-y-2 md:max-w-[180px]">
                <Label for="tts-scheme" class="text-gray-300">Protocol</Label>
                <Select v-model="form.wsScheme">
                  <SelectTrigger
                    id="tts-scheme"
                    class="h-9 border-gray-700 bg-gray-900 text-white data-[placeholder]:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent class="border-gray-700 bg-gray-900 text-white">
                    <SelectItem value="ws" class="text-gray-200 focus:bg-gray-800 focus:text-white">
                      WS
                    </SelectItem>
                    <SelectItem
                      value="wss"
                      class="text-gray-200 focus:bg-gray-800 focus:text-white"
                    >
                      WSS
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div class="min-w-0 space-y-2">
              <div class="flex items-center justify-between gap-2">
                <Label for="tts-url" class="text-gray-300">WS URL</Label>
                <button
                  type="button"
                  @click="fillWsUrlFromSelectedStream"
                  :disabled="!selectedStream || loadingStreams"
                  class="inline-flex h-7 shrink-0 items-center justify-center rounded-md border border-gray-700 px-2 text-[11px] text-gray-300 transition-colors hover:bg-gray-800 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
                >
                  Auto Fill
                </button>
              </div>
              <Input
                id="tts-url"
                v-model="form.url"
                type="text"
                :placeholder="wsUrlPlaceholder"
                class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
              />
              <p v-if="autoFillHint" class="break-all text-xs text-gray-500">{{ autoFillHint }}</p>
            </div>
            <div class="grid min-w-0 gap-3 md:grid-cols-2">
              <div class="min-w-0 space-y-2">
                <Label for="tts-sample-rate" class="text-gray-300">Sample Rate</Label>
                <Input
                  id="tts-sample-rate"
                  v-model="form.sampleRate"
                  type="number"
                  min="8000"
                  step="1000"
                  class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
                />
              </div>
              <div class="min-w-0 space-y-2">
                <Label for="tts-chunk-ms" class="text-gray-300">Chunk (ms)</Label>
                <Input
                  id="tts-chunk-ms"
                  v-model="form.chunkMs"
                  type="number"
                  min="20"
                  step="10"
                  class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
                />
              </div>
            </div>
            <label class="inline-flex cursor-pointer items-center gap-2 text-xs text-gray-400">
              <input
                v-model="form.realtime"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-600 bg-gray-800 text-blue-600 focus:ring-blue-500"
              />
              Realtime push
            </label>
            <label class="inline-flex cursor-pointer items-center gap-2 text-xs text-gray-400">
              <input
                v-model="form.insecureTls"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-600 bg-gray-800 text-blue-600 focus:ring-blue-500"
              />
              Allow insecure TLS handshake
            </label>
          </div>
        </div>
      </details>

      <details
        class="min-w-0 rounded-md border border-gray-800 bg-gray-900/50 p-3 text-sm text-gray-300"
      >
        <summary class="cursor-pointer select-none text-xs text-gray-400">Advanced</summary>
        <div class="mt-3 grid min-w-0 gap-3">
          <div class="min-w-0 space-y-2">
            <Label for="tts-proxy" class="text-gray-300">Proxy (optional)</Label>
            <Input
              id="tts-proxy"
              v-model="form.proxy"
              type="text"
              placeholder="http://127.0.0.1:7890"
              class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
            />
          </div>
          <div class="grid min-w-0 gap-3 md:grid-cols-2">
            <div class="min-w-0 space-y-2">
              <Label for="tts-connect-timeout" class="text-gray-300">Connect Timeout (s)</Label>
              <Input
                id="tts-connect-timeout"
                v-model="form.connectTimeout"
                type="number"
                min="1"
                step="1"
                class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
              />
            </div>
            <div class="min-w-0 space-y-2">
              <Label for="tts-receive-timeout" class="text-gray-300">Receive Timeout (s)</Label>
              <Input
                id="tts-receive-timeout"
                v-model="form.receiveTimeout"
                type="number"
                min="1"
                step="1"
                class="border-gray-700 bg-gray-900 text-white placeholder:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
              />
            </div>
          </div>
        </div>
      </details>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { RefreshCw } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import * as api from '@/services/api'

type PushState = {
  type: 'success' | 'error' | 'info'
  message: string
}

type TargetType = 'backchannel' | 'ipwebcam'
type WsScheme = 'ws' | 'wss'

type VoiceOption = {
  value: string
  gender: 'female' | 'male'
  dialect: string
  region: string
  description: string
}

type TtsFormCache = {
  text: string
  voice: string
  rate: string
  pitch: string
  volume: string
  targetType: TargetType
  wsScheme: WsScheme
  dst: string
  url: string
  clearTextOnSuccess: boolean
  sampleRate: string
  chunkMs: string
  realtime: boolean
  insecureTls: boolean
  proxy: string
  connectTimeout: string
  receiveTimeout: string
}

type TtsPersistedConfig = {
  syncDstWithStream: boolean
  streams: Record<string, TtsFormCache>
}

const TTS_PUSH_CONFIG_KEY = 'tts-push-panel-config-v1'

const DEFAULT_FORM_STATE: TtsFormCache = {
  text: '',
  voice: 'zh-CN-XiaoxiaoNeural',
  rate: '+0%',
  pitch: '+0Hz',
  volume: '+0%',
  targetType: 'backchannel',
  wsScheme: 'ws',
  dst: '',
  url: '',
  clearTextOnSuccess: true,
  sampleRate: '24000',
  chunkMs: '40',
  realtime: true,
  insecureTls: true,
  proxy: '',
  connectTimeout: '10',
  receiveTimeout: '90',
}

const VOICE_OPTIONS: VoiceOption[] = [
  {
    value: 'zh-CN-XiaoxiaoNeural',
    gender: 'female',
    dialect: 'Mandarin',
    region: 'China',
    description: 'Gentle natural female voice',
  },
  {
    value: 'zh-CN-XiaoyiNeural',
    gender: 'female',
    dialect: 'Mandarin',
    region: 'China',
    description: 'Lively young female voice',
  },
  {
    value: 'zh-CN-YunjianNeural',
    gender: 'male',
    dialect: 'Mandarin',
    region: 'China',
    description: 'Steady mature male voice',
  },
  {
    value: 'zh-CN-YunxiNeural',
    gender: 'male',
    dialect: 'Mandarin',
    region: 'China',
    description: 'Natural neutral male voice',
  },
  {
    value: 'zh-CN-YunxiaNeural',
    gender: 'male',
    dialect: 'Mandarin',
    region: 'China',
    description: 'Warm friendly male voice',
  },
  {
    value: 'zh-CN-YunyangNeural',
    gender: 'male',
    dialect: 'Mandarin',
    region: 'China',
    description: 'Lively young male voice',
  },
  {
    value: 'zh-HK-HiuGaaiNeural',
    gender: 'female',
    dialect: 'Cantonese',
    region: 'Hong Kong',
    description: 'Natural Cantonese female voice',
  },
  {
    value: 'zh-HK-HiuMaanNeural',
    gender: 'female',
    dialect: 'Cantonese',
    region: 'Hong Kong',
    description: 'Natural Cantonese female voice',
  },
  {
    value: 'zh-HK-WanLungNeural',
    gender: 'male',
    dialect: 'Cantonese',
    region: 'Hong Kong',
    description: 'Natural Cantonese male voice',
  },
]

const props = withDefaults(
  defineProps<{
    selectedStream: string
    streamSourceUrls?: Record<string, string>
    loadingStreams?: boolean
  }>(),
  {
    streamSourceUrls: () => ({}),
    loadingStreams: false,
  },
)

const syncDstWithStream = ref(true)
const autoFillHint = ref('')
const pushState = ref<PushState | null>(null)
const pushing = ref(false)
const autoFillOnceKey = ref('')
const streamScopedCache = ref<Record<string, TtsFormCache>>({})
const configRestored = ref(false)

const form = reactive<TtsFormCache>({ ...DEFAULT_FORM_STATE })

const selectedVoiceDescription = computed(() => {
  const voice = VOICE_OPTIONS.find((item) => item.value === form.voice)
  return voice ? `${voice.value}: ${voice.description}` : ''
})

const wsUrlPlaceholder = computed(() => {
  return `${form.wsScheme}://192.168.123.42:8080/audioin.wav`
})

const modeSummary = computed(() => {
  if (form.targetType === 'backchannel') {
    return `Mode: Backchannel | dst: ${form.dst.trim() || props.selectedStream || '(empty)'}`
  }
  return `Mode: IP Webcam | protocol: ${form.wsScheme.toUpperCase()} | url: ${form.url.trim() ? 'set' : 'empty'}`
})

const pushStateClass = computed(() => {
  if (!pushState.value) {
    return 'border-gray-800 bg-gray-900/50 text-gray-500 [&_p]:text-gray-500'
  }
  if (pushState.value.type === 'error') {
    return 'border-red-800 bg-red-900/40 text-red-200 [&_p]:text-red-200'
  }
  if (pushState.value.type === 'success') {
    return 'border-green-800 bg-green-900/30 text-green-200 [&_p]:text-green-200'
  }
  return 'border-blue-800 bg-blue-900/30 text-blue-200 [&_p]:text-blue-200'
})

const pushStateMessage = computed(() => {
  if (!pushState.value) return 'Ready. Push result will appear here.'
  return pushState.value.message
})

const canSubmit = computed(() => {
  if (pushing.value || !form.text.trim()) return false
  if (form.targetType === 'backchannel') return !!form.dst.trim()
  return !!form.url.trim()
})

function trimOptional(value: string): string | undefined {
  const trimmed = value.trim()
  return trimmed || undefined
}

function toOptionalPositiveInt(value: string | number): number | undefined {
  const n = Number(value)
  if (!Number.isFinite(n) || n <= 0) return undefined
  return Math.floor(n)
}

function toIpWebcamWsUrlFromSource(sourceUrl: string, scheme: WsScheme): string | undefined {
  const trimmed = sourceUrl.trim()
  const normalized = trimmed
    .replace(/^\-\s*/, '')
    .replace(/^['"]|['"]$/g, '')
    .trim()
  if (!normalized) return undefined

  try {
    const parsed = new URL(normalized)
    if (parsed.host) {
      const auth = parsed.username
        ? `${parsed.username}${parsed.password ? `:${parsed.password}` : ''}@`
        : ''
      return `${scheme}://${auth}${parsed.host}/audioin.wav`
    }
  } catch (_) {
    // Fallback to manual parsing below.
  }

  const schemePos = normalized.indexOf('://')
  if (schemePos > 0) {
    const authorityRaw =
      normalized
        .slice(schemePos + 3)
        .split(/[/?#\s,]/)[0]
        ?.trim() || ''
    const authority = authorityRaw.replace(/^['"]|['"]$/g, '')
    if (authority) {
      return `${scheme}://${authority}/audioin.wav`
    }
  }

  const candidates: Array<{ protocol: string; authority: string }> = []
  const urlPattern = /([a-zA-Z][a-zA-Z0-9+.-]*):\/\/([^/\s?#,]+)/g
  let match: RegExpExecArray | null = null
  while ((match = urlPattern.exec(normalized)) !== null) {
    const protocol = (match[1] || '').toLowerCase()
    const authority = (match[2] || '').trim()
    if (!authority) continue
    candidates.push({ protocol, authority })
  }

  if (!candidates.length) return undefined

  const preferred = ['rtsp', 'rtsps', 'http', 'https', 'ws', 'wss']
  candidates.sort((a, b) => {
    const ai = preferred.indexOf(a.protocol)
    const bi = preferred.indexOf(b.protocol)
    const arank = ai >= 0 ? ai : preferred.length
    const brank = bi >= 0 ? bi : preferred.length
    return arank - brank
  })

  const picked = candidates[0]
  if (!picked) return undefined
  return `${scheme}://${picked.authority}/audioin.wav`
}

function fillWsUrlFromSelectedStream() {
  const streamName = props.selectedStream.trim()

  if (!streamName) {
    autoFillHint.value = 'Please select a stream first.'
    return
  }

  const sourceUrl = props.streamSourceUrls?.[streamName]
  if (!sourceUrl || typeof sourceUrl !== 'string') {
    autoFillHint.value = 'No source URL found in config for selected stream.'
    return
  }

  const derivedWsUrl = toIpWebcamWsUrlFromSource(sourceUrl, form.wsScheme)
  if (!derivedWsUrl) {
    autoFillHint.value = 'Failed to build URL from stream source URL.'
    return
  }

  form.url = derivedWsUrl
  autoFillHint.value = `Auto filled from source URL: ${sourceUrl}`
}

function normalizeTargetType(value: unknown, fallback: TargetType): TargetType {
  if (value === 'backchannel' || value === 'ipwebcam') return value
  return fallback
}

function normalizeWsScheme(value: unknown, fallback: WsScheme): WsScheme {
  if (value === 'ws' || value === 'wss') return value
  return fallback
}

function normalizeFormState(input: unknown, fallback: TtsFormCache): TtsFormCache {
  const src = input && typeof input === 'object' ? (input as Record<string, unknown>) : {}
  return {
    text: typeof src.text === 'string' ? src.text : fallback.text,
    voice: typeof src.voice === 'string' ? src.voice : fallback.voice,
    rate: typeof src.rate === 'string' ? src.rate : fallback.rate,
    pitch: typeof src.pitch === 'string' ? src.pitch : fallback.pitch,
    volume: typeof src.volume === 'string' ? src.volume : fallback.volume,
    targetType: normalizeTargetType(src.targetType, fallback.targetType),
    wsScheme: normalizeWsScheme(src.wsScheme, fallback.wsScheme),
    dst: typeof src.dst === 'string' ? src.dst : fallback.dst,
    url: typeof src.url === 'string' ? src.url : fallback.url,
    clearTextOnSuccess:
      typeof src.clearTextOnSuccess === 'boolean'
        ? src.clearTextOnSuccess
        : fallback.clearTextOnSuccess,
    sampleRate: typeof src.sampleRate === 'string' ? src.sampleRate : fallback.sampleRate,
    chunkMs: typeof src.chunkMs === 'string' ? src.chunkMs : fallback.chunkMs,
    realtime: typeof src.realtime === 'boolean' ? src.realtime : fallback.realtime,
    insecureTls: typeof src.insecureTls === 'boolean' ? src.insecureTls : fallback.insecureTls,
    proxy: typeof src.proxy === 'string' ? src.proxy : fallback.proxy,
    connectTimeout:
      typeof src.connectTimeout === 'string' ? src.connectTimeout : fallback.connectTimeout,
    receiveTimeout:
      typeof src.receiveTimeout === 'string' ? src.receiveTimeout : fallback.receiveTimeout,
  }
}

function readFormState(): TtsFormCache {
  return {
    text: form.text,
    voice: form.voice,
    rate: form.rate,
    pitch: form.pitch,
    volume: form.volume,
    targetType: form.targetType,
    wsScheme: form.wsScheme,
    dst: form.dst,
    url: form.url,
    clearTextOnSuccess: form.clearTextOnSuccess,
    sampleRate: form.sampleRate,
    chunkMs: form.chunkMs,
    realtime: form.realtime,
    insecureTls: form.insecureTls,
    proxy: form.proxy,
    connectTimeout: form.connectTimeout,
    receiveTimeout: form.receiveTimeout,
  }
}

function applyFormState(state: TtsFormCache) {
  Object.assign(form, state)
}

function saveConfig() {
  if (!configRestored.value) return

  const activeStream = props.selectedStream.trim()
  if (activeStream) {
    streamScopedCache.value[activeStream] = readFormState()
  }

  const payload: TtsPersistedConfig = {
    syncDstWithStream: syncDstWithStream.value,
    streams: { ...streamScopedCache.value },
  }
  localStorage.setItem(TTS_PUSH_CONFIG_KEY, JSON.stringify(payload))
}

function restoreConfig() {
  try {
    const raw = localStorage.getItem(TTS_PUSH_CONFIG_KEY)
    if (!raw) return
    const data = JSON.parse(raw) as Record<string, unknown>
    if (typeof data.syncDstWithStream === 'boolean') {
      syncDstWithStream.value = data.syncDstWithStream
    }
    const streamsRaw =
      data.streams && typeof data.streams === 'object'
        ? (data.streams as Record<string, unknown>)
        : {}
    const normalizedStreams: Record<string, TtsFormCache> = {}
    for (const [streamName, value] of Object.entries(streamsRaw)) {
      normalizedStreams[streamName] = normalizeFormState(value, DEFAULT_FORM_STATE)
    }
    streamScopedCache.value = normalizedStreams
  } catch (_) {
    // Ignore invalid persisted config.
  }
}

function tryAutoFillUrlOnceWhenEmpty() {
  const streamName = props.selectedStream.trim()
  if (form.targetType !== 'ipwebcam') return
  if (!streamName) return
  if (form.url.trim()) return

  const key = `${streamName}|${form.wsScheme}`
  if (autoFillOnceKey.value === key) return
  autoFillOnceKey.value = key
  fillWsUrlFromSelectedStream()
}

async function submitTts() {
  const text = form.text.trim()
  if (!text) {
    pushState.value = { type: 'error', message: 'Text is required.' }
    return
  }

  const payload: api.TtsPushPayload = {
    text,
    voice: trimOptional(form.voice),
    rate: trimOptional(form.rate),
    pitch: trimOptional(form.pitch),
    volume: trimOptional(form.volume),
    proxy: trimOptional(form.proxy),
  }

  const connectTimeout = toOptionalPositiveInt(form.connectTimeout)
  const receiveTimeout = toOptionalPositiveInt(form.receiveTimeout)
  if (connectTimeout !== undefined) payload.connect_timeout = connectTimeout
  if (receiveTimeout !== undefined) payload.receive_timeout = receiveTimeout

  if (form.targetType === 'backchannel') {
    const dst = form.dst.trim()
    if (!dst) {
      pushState.value = { type: 'error', message: 'Destination stream (dst) is required.' }
      return
    }
    payload.target_type = 'backchannel'
    payload.dst = dst
    payload.format = 'wav'
  } else {
    const url = form.url.trim()
    if (!url) {
      pushState.value = { type: 'error', message: 'WS/WSS URL is required.' }
      return
    }
    payload.target_type = 'ipwebcam'
    payload.url = url

    const sampleRate = toOptionalPositiveInt(form.sampleRate)
    const chunkMs = toOptionalPositiveInt(form.chunkMs)
    if (sampleRate !== undefined) payload.sample_rate = sampleRate
    if (chunkMs !== undefined) payload.chunk_ms = chunkMs
    payload.realtime = !!form.realtime
    payload.insecure_tls = !!form.insecureTls
  }

  pushing.value = true
  pushState.value = { type: 'info', message: 'Pushing TTS...' }
  try {
    const result = await api.pushTts(payload)
    let detail = ''
    if (typeof result === 'string') {
      detail = result
    } else if (result && typeof result === 'object') {
      detail = String((result as any).message || (result as any).result || '').trim()
    }
    pushState.value = {
      type: 'success',
      message: detail ? `TTS pushed successfully: ${detail}` : 'TTS pushed successfully.',
    }
    if (form.clearTextOnSuccess) {
      form.text = ''
    }
  } catch (e: any) {
    pushState.value = {
      type: 'error',
      message: e?.message || 'Failed to push TTS',
    }
  } finally {
    pushing.value = false
  }
}

function onTextKeydown(event: KeyboardEvent) {
  if (event.isComposing) return
  if (event.key !== 'Enter') return
  if (event.shiftKey) return

  event.preventDefault()
  void submitTts()
}

function applyStreamScopedConfig(streamName: string) {
  const stream = streamName.trim()
  const scoped = stream ? streamScopedCache.value[stream] : undefined
  const nextState = normalizeFormState(scoped, DEFAULT_FORM_STATE)
  if (syncDstWithStream.value) {
    nextState.dst = stream || ''
  }
  applyFormState(nextState)
}

watch(
  () => props.selectedStream,
  (streamName) => {
    applyStreamScopedConfig(streamName)
    autoFillOnceKey.value = ''
    tryAutoFillUrlOnceWhenEmpty()
  },
  { immediate: true },
)

watch(
  () => form.targetType,
  (targetType) => {
    if (targetType === 'backchannel' && syncDstWithStream.value) {
      form.dst = props.selectedStream || ''
    }
    if (targetType !== 'ipwebcam') {
      autoFillHint.value = ''
      autoFillOnceKey.value = ''
    }
    if (targetType === 'ipwebcam') {
      tryAutoFillUrlOnceWhenEmpty()
    }
  },
)

watch(
  form,
  () => {
    saveConfig()
  },
  { deep: true },
)

watch(syncDstWithStream, () => {
  if (syncDstWithStream.value) {
    form.dst = props.selectedStream || ''
  }
  saveConfig()
})

watch(
  [() => props.selectedStream, () => form.targetType, () => form.wsScheme, () => form.url],
  () => {
    tryAutoFillUrlOnceWhenEmpty()
  },
)

watch(
  () => {
    const streamName = props.selectedStream.trim()
    return streamName ? props.streamSourceUrls?.[streamName] : ''
  },
  () => {
    tryAutoFillUrlOnceWhenEmpty()
  },
)

onMounted(() => {
  restoreConfig()
  applyStreamScopedConfig(props.selectedStream)
  configRestored.value = true
  tryAutoFillUrlOnceWhenEmpty()
})
</script>
