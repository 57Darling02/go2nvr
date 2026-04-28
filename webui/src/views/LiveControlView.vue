<template>
  <div class="flex h-full min-h-0 flex-col overflow-hidden bg-gray-900 text-white">
    <header class="shrink-0 border-b border-gray-800 bg-gray-900 px-4 py-3 md:px-6 md:py-4">
      <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <h1 class="text-lg font-semibold text-white md:text-xl">Live Control</h1>

        <div class="flex w-full min-w-0 flex-wrap items-center gap-2 md:w-auto md:justify-end">
          <div class="flex min-w-0 flex-1 items-center gap-2 md:min-w-[280px] md:flex-none">
            <Label for="live-stream-select" class="shrink-0 text-xs text-gray-400">Stream</Label>
            <Select v-model="selectedStream" :disabled="streams.length === 0">
              <SelectTrigger
                id="live-stream-select"
                class="h-9 border-gray-700 bg-gray-800 text-white data-[placeholder]:text-gray-500 focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
              >
                <SelectValue
                  :placeholder="streams.length ? 'Select stream' : 'No stream available'"
                />
              </SelectTrigger>
              <SelectContent class="border-gray-700 bg-gray-900 text-white">
                <SelectItem
                  v-for="stream in streams"
                  :key="stream"
                  :value="stream"
                  class="text-gray-200 focus:bg-gray-800 focus:text-white"
                >
                  {{ stream }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <button
            @click="refreshStreams"
            class="rounded-full p-2 text-gray-400 transition-colors hover:bg-gray-800 hover:text-white"
            title="Refresh Streams"
          >
            <RefreshCw class="h-5 w-5" :class="{ 'animate-spin': loading }" />
          </button>
        </div>
      </div>
    </header>

    <div class="min-h-0 flex-1 p-2 md:p-4">
      <div
        v-if="loading && streams.length === 0"
        class="flex h-full items-center justify-center text-gray-500"
      >
        Loading streams...
      </div>

      <div
        v-else
        class="grid h-full min-h-0 grid-cols-1 grid-rows-2 gap-3 md:grid-cols-[7fr_3fr] md:grid-rows-1 md:gap-4"
      >
        <section class="min-h-0 overflow-hidden">
          <LiveStreamCard
            :src="selectedStream || undefined"
            :title="selectedStream"
            :show-dashboard-tools="true"
            :trigger-label="
              selectedStream && isTriggerEnabled(selectedStream)
                ? getTriggerType(selectedStream)
                : undefined
            "
            :status-dot-class="selectedStream ? getStatusClass(selectedStream) : undefined"
            :record-toggle-title="selectedStream ? getToggleTitle(selectedStream) : undefined"
            :recordings-to="recordingsLink"
            mode-storage-key="live-control-stream-mode"
            class="h-full w-full transition-all hover:border-gray-700 hover:shadow-md"
            @open-links="openLinks"
            @toggle-record="toggleRecord"
            @open-record-settings="openRecordConfig"
            @delete-stream="removeStream"
          />
        </section>

        <section class="min-h-0 overflow-hidden rounded-lg border border-gray-800 bg-gray-950">
          <div class="flex h-full min-h-0 flex-col">
            <div class="shrink-0 border-b border-gray-800 p-4">
              <h2 class="text-sm font-semibold text-white">Control Card</h2>
            </div>

            <ScrollArea class="min-h-0 flex-1">
              <div class="space-y-3 p-4">
                <TtsPushPanel
                  :selected-stream="selectedStream"
                  :stream-source-urls="streamSourceUrls"
                  :loading-streams="loading"
                />
              </div>
            </ScrollArea>
          </div>
        </section>
      </div>
    </div>

    <AlertDialog :open="showDeleteDialog" @update:open="showDeleteDialog = $event">
      <AlertDialogContent class="bg-gray-900 border-gray-700 text-white">
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Stream?</AlertDialogTitle>
          <AlertDialogDescription class="text-gray-400">
            This will permanently delete stream
            <span class="text-red-400 font-medium">"{{ pendingDeleteStream }}"</span>.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel
            class="bg-gray-800 border-gray-600 text-gray-200 hover:bg-gray-700 hover:text-white"
          >
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction
            class="bg-red-600 text-white hover:bg-red-500"
            @click="confirmRemoveStream"
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
    <StreamLinksModal v-model:open="showLinksModal" :stream="selectedStream" />
    <StreamRecordModal
      v-model:open="showRecordModal"
      :stream="selectedStreamForRecord"
      @saved="refreshStreams"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RefreshCw } from 'lucide-vue-next'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import LiveStreamCard from '@/components/live/LiveStreamCard.vue'
import TtsPushPanel from '@/components/live/TtsPushPanel.vue'
import StreamLinksModal from '@/components/StreamLinksModal.vue'
import StreamRecordModal from '@/components/StreamRecordModal.vue'
import {
  getStatusClass as getRecordingStatusClass,
  getStreamStatus as getRecordingStreamStatus,
  getToggleTitle as getRecordingToggleTitle,
  getTriggerType as getRecordingTriggerType,
  isTriggerEnabled as isRecordingTriggerEnabled,
  parseRecordingRuntimeState,
  type StreamRuntimeStateMap,
} from '@/lib/recording-state'
import * as api from '@/services/api'

const streams = ref<string[]>([])
const streamSourceUrls = ref<Record<string, string>>({})
const recordingState = ref<StreamRuntimeStateMap>({})
const loading = ref(false)
const selectedStream = ref('')
const selectedStreamForRecord = ref('')
const pendingDeleteStream = ref('')
const showLinksModal = ref(false)
const showRecordModal = ref(false)
const showDeleteDialog = ref(false)
let pollInterval: number | undefined

const recordingsLink = computed(() => {
  return selectedStream.value
    ? `/recordings?src=${encodeURIComponent(selectedStream.value)}`
    : '/recordings'
})

function getStreamStatus(stream: string) {
  return getRecordingStreamStatus(recordingState.value, stream)
}

function isTriggerEnabled(stream: string) {
  return isRecordingTriggerEnabled(recordingState.value, stream)
}

function getTriggerType(stream: string) {
  return getRecordingTriggerType(recordingState.value, stream)
}

function getStatusClass(stream: string) {
  return getRecordingStatusClass(recordingState.value, stream)
}

function getToggleTitle(stream: string) {
  return getRecordingToggleTitle(recordingState.value, stream)
}

async function fetchRecordingStatus() {
  try {
    const res = await api.getRecordingStatus()
    recordingState.value = parseRecordingRuntimeState(res)
  } catch (_) {
    // Keep old runtime state if request fails.
  }
}

async function refreshStreams() {
  loading.value = true
  try {
    const [configYaml, streamsInfo] = await Promise.all([
      api.getConfig(),
      api.getAllStreamsInfo(),
      fetchRecordingStatus(),
    ])
    const parsedSources = parseStreamSourcesFromConfig(configYaml)
    streamSourceUrls.value = mergeSourceMapWithStreamsInfo(parsedSources, streamsInfo || {})

    const list = Object.keys(streamSourceUrls.value).sort()
    streams.value = list
    if (!list.includes(selectedStream.value)) {
      selectedStream.value = list[0] || ''
    }

    // Fallback when config parsing yields no stream entries.
    if (list.length === 0) {
      const fallbackStreams = await api.getStreams()
      streams.value = fallbackStreams
      if (!fallbackStreams.includes(selectedStream.value)) {
        selectedStream.value = fallbackStreams[0] || ''
      }
    }
  } finally {
    loading.value = false
  }
}

function hasUrlToken(value: string): boolean {
  return /[a-zA-Z][a-zA-Z0-9+.-]*:\/\/[^/\s?#,]+/.test(value)
}

function chooseBetterSource(currentValue: string, nextValue: string): string {
  if (!currentValue) return nextValue
  const currentHasUrl = hasUrlToken(currentValue)
  const nextHasUrl = hasUrlToken(nextValue)
  if (!currentHasUrl && nextHasUrl) return nextValue
  return currentValue
}

function mergeSourceMapWithStreamsInfo(
  parsedSources: Record<string, string>,
  streamsInfo: api.StreamInfo,
): Record<string, string> {
  const merged: Record<string, string> = { ...parsedSources }
  for (const [name, info] of Object.entries(streamsInfo || {})) {
    const producer = info?.producers?.[0]
    const fromProducerUrl = typeof producer?.url === 'string' ? producer.url.trim() : ''
    const fromRemoteAddr =
      typeof producer?.remote_addr === 'string' ? producer.remote_addr.trim() : ''
    const fallback = fromProducerUrl || (fromRemoteAddr ? `rtsp://${fromRemoteAddr}` : '')
    if (!fallback) continue
    merged[name] = chooseBetterSource(merged[name] || '', fallback)
  }
  return merged
}

async function toggleRecord() {
  const stream = selectedStream.value
  if (!stream) return
  try {
    const status = getStreamStatus(stream)
    if (status === 'recording') {
      await api.stopRecording(stream)
    } else {
      await api.startRecording(stream)
    }
    await fetchRecordingStatus()
  } catch (_) {
    // Ignore transient toggle errors in UI layer.
  }
}

function openLinks() {
  if (!selectedStream.value) return
  showLinksModal.value = true
}

function openRecordConfig() {
  if (!selectedStream.value) return
  selectedStreamForRecord.value = selectedStream.value
  showRecordModal.value = true
}

function removeStream() {
  if (!selectedStream.value) return
  pendingDeleteStream.value = selectedStream.value
  showDeleteDialog.value = true
}

async function confirmRemoveStream() {
  if (!pendingDeleteStream.value) return
  try {
    await api.deleteStream(pendingDeleteStream.value)
    showDeleteDialog.value = false
    pendingDeleteStream.value = ''
    await refreshStreams()
  } catch (_) {
    // Keep dialog state if delete fails.
  }
}

function stripQuoted(value: string): string {
  const trimmed = value.trim()
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1).trim()
  }
  return trimmed
}

function parseStreamSourcesFromConfig(configYaml: string): Record<string, string> {
  const result: Record<string, string> = {}
  const lines = configYaml.split(/\r?\n/)

  let inStreams = false
  let streamsIndent = 0
  let currentStream = ''

  for (const rawLine of lines) {
    if (!rawLine.trim() || rawLine.trim().startsWith('#')) continue

    const indent = rawLine.length - rawLine.trimStart().length
    const trimmed = rawLine.trim()

    if (!inStreams) {
      if (trimmed === 'streams:') {
        inStreams = true
        streamsIndent = indent
      }
      continue
    }

    if (indent <= streamsIndent && !trimmed.startsWith('-')) {
      break
    }

    const streamLineMatch = /^([^:#][^:]*):\s*(.*)$/.exec(trimmed)
    if (indent === streamsIndent + 2 && streamLineMatch) {
      const streamName = streamLineMatch[1]?.trim()
      if (!streamName) continue
      currentStream = streamName
      const inlineValue = stripQuoted(streamLineMatch[2] || '')
      if (!(currentStream in result)) {
        result[currentStream] = ''
      }
      if (inlineValue) {
        result[currentStream] = chooseBetterSource(result[currentStream] || '', inlineValue)
      }
      continue
    }

    if (!currentStream) continue

    if (indent >= streamsIndent + 4 && trimmed.startsWith('-')) {
      const listValue = stripQuoted(trimmed.replace(/^-+\s*/, ''))
      if (listValue) {
        result[currentStream] = chooseBetterSource(result[currentStream] || '', listValue)
      }
    }
  }

  return result
}

onMounted(() => {
  refreshStreams()
  pollInterval = window.setInterval(fetchRecordingStatus, 1000)
})

onUnmounted(() => {
  if (pollInterval) {
    clearInterval(pollInterval)
    pollInterval = undefined
  }
})
</script>
