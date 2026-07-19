<template>
  <div class="flex flex-col h-full relative">
    <header
      class="flex flex-row items-center justify-between gap-3 border-b border-gray-800 bg-gray-900 px-4 py-3 sm:px-6 sm:py-4"
    >
      <h1 class="min-w-0 truncate text-lg font-semibold text-white sm:text-xl">Dashboard</h1>

      <div class="flex shrink-0 items-center gap-2 sm:justify-end">
        <div class="flex items-center rounded-md border border-gray-700 bg-gray-800 p-0.5">
          <button
            @click="showGlobalConfigModal = true"
            class="rounded-full p-2 text-gray-400 transition-colors hover:bg-gray-800 hover:text-white"
            title="Global Recording Settings"
          >
            <Database class="w-5 h-5" />
          </button>
          <button
            @click="showAddModal = true"
            class="rounded-full p-2 text-gray-400 transition-colors hover:bg-gray-800 hover:text-white"
            title="Add Stream"
          >
            <Plus class="w-5 h-5" />
          </button>
          <button
            @click="refresh"
            class="rounded-full p-2 text-gray-400 transition-colors hover:bg-gray-800 hover:text-white"
            title="Refresh"
          >
            <RefreshCw class="w-5 h-5" :class="{ 'animate-spin': loading }" />
          </button>
        </div>
      </div>
    </header>

    <div class="flex-1 p-2 sm:p-4 lg:p-6 min-h-0 overflow-y-auto overflow-x-hidden">
      <Alert
        v-show="deleteErrorMessage"
        variant="destructive"
        class="mb-4 border-red-800 bg-red-900/40 text-red-200 [&_p]:text-red-200"
      >
        <AlertDescription>{{ deleteErrorMessage }}</AlertDescription>
      </Alert>

      <div
        v-if="loading && streams.length === 0"
        class="flex items-center justify-center h-full text-gray-500"
      >
        Loading streams...
      </div>

      <div
        v-else-if="streams.length === 0"
        class="flex flex-col items-center justify-center h-full text-gray-500"
      >
        <VideoOff class="w-12 h-12 mb-4 opacity-50" />
        <p>No streams found.</p>
        <button
          @click="showAddModal = true"
          class="mt-4 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-500 transition-colors"
        >
          Add Stream
        </button>
      </div>

      <div
        v-else
        ref="gridContainerRef"
        class="grid min-h-full"
        :style="gridStyle"
        :key="refreshKey"
      >
        <LiveStreamCard
          v-for="stream in streams"
          :key="stream"
          :src="stream"
          :title="stream"
          :mode-storage-key="`dashboard-stream-mode:${stream}`"
          :show-dashboard-tools="true"
          :trigger-label="isTriggerEnabled(stream) ? getTriggerType(stream) : undefined"
          :status-dot-class="getStatusClass(stream)"
          :record-toggle-title="getToggleTitle(stream)"
          :recordings-to="`/recordings?src=${encodeURIComponent(stream)}`"
          @open-links="openLinks(stream)"
          @toggle-record="toggleRecord(stream)"
          @open-record-settings="openRecordConfig(stream)"
          @delete-stream="removeStream(stream)"
          class="h-full w-full transition-all hover:border-gray-700 hover:shadow-md"
        />
      </div>
    </div>

    <!-- Modals -->
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
    <AddStreamModal v-model:open="showAddModal" @added="refresh" />
    <StreamLinksModal v-model:open="showLinksModal" :stream="selectedStream" />
    <StreamRecordModal
      v-model:open="showRecordModal"
      :stream="selectedStreamForRecord"
      @saved="refresh"
    />
    <GlobalRecordConfigModal v-model:open="showGlobalConfigModal" />

    <!-- Offline Overlay -->
    <div
      v-if="isServerOffline"
      class="fixed inset-0 z-100 flex flex-col items-center justify-center bg-gray-900/90 backdrop-blur-md"
    >
      <div
        class="bg-gray-800 p-6 sm:p-8 rounded-lg shadow-2xl border border-gray-700 text-center max-w-md w-full mx-4"
      >
        <div
          class="w-16 h-16 bg-red-500/10 rounded-full flex items-center justify-center mx-auto mb-4"
        >
          <Power class="w-8 h-8 text-red-500" />
        </div>
        <h2 class="text-2xl font-bold text-white mb-2">Server Disconnected</h2>
        <p class="text-gray-400 mb-6">
          The server has been stopped or is unreachable. Please restart the service manually if you
          stopped it.
        </p>

        <div class="flex flex-col gap-3">
          <div
            v-if="reconnecting"
            class="flex items-center justify-center text-blue-400 text-sm font-medium animate-pulse py-2"
          >
            <RefreshCw class="w-4 h-4 mr-2 animate-spin" />
            Checking connection...
          </div>
          <button
            v-else
            @click="checkConnection"
            class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded transition-colors w-full flex items-center justify-center font-medium"
          >
            <RefreshCw class="w-4 h-4 mr-2" />
            Retry Connection
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { RefreshCw, VideoOff, Plus, Power, Database } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import LiveStreamCard from '@/components/live/LiveStreamCard.vue'
import AddStreamModal from '@/components/AddStreamModal.vue'
import StreamLinksModal from '@/components/StreamLinksModal.vue'
import StreamRecordModal from '@/components/StreamRecordModal.vue'
import GlobalRecordConfigModal from '@/components/GlobalRecordConfigModal.vue'
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
import { apiURL } from '@/lib/base-url'

const streams = ref<string[]>([])
const loading = ref(false)
const recordingState = ref<StreamRuntimeStateMap>({})
const showAddModal = ref(false)
const showLinksModal = ref(false)
const showRecordModal = ref(false)
const showGlobalConfigModal = ref(false)
const showDeleteDialog = ref(false)
const selectedStream = ref('')
const selectedStreamForRecord = ref('')
const pendingDeleteStream = ref('')
const deleteErrorMessage = ref('')
const isServerOffline = ref(false)
const reconnecting = ref(false)
const refreshKey = ref(0)
const gridContainerRef = ref<HTMLElement | null>(null)
const containerSize = ref({ width: 0, height: 0 })
let pollInterval: number | undefined
let connectionCheckInterval: number | undefined
let gridResizeObserver: ResizeObserver | undefined
let deleteErrorTimer: number | undefined

const GRID_GAP = 16
const CONTROL_BAR_HEIGHT = 52
const VIDEO_ASPECT = 16 / 9
const MIN_VIDEO_WIDTH = 240

const gridLayout = computed(() => {
  const count = streams.value.length
  const width = containerSize.value.width
  const height = containerSize.value.height
  if (count <= 0 || width <= 0 || height <= 0) {
    return {
      cols: 1,
      rows: 1,
      tileWidth: width || MIN_VIDEO_WIDTH,
      tileHeight: Math.floor((width || MIN_VIDEO_WIDTH) / VIDEO_ASPECT) + CONTROL_BAR_HEIGHT,
      shouldScroll: false,
    }
  }

  const maxCols = Math.max(
    1,
    Math.min(count, Math.floor((width + GRID_GAP) / (MIN_VIDEO_WIDTH + GRID_GAP))),
  )
  let best = {
    cols: 1,
    rows: count,
    tileWidth: MIN_VIDEO_WIDTH,
    tileHeight: Math.floor(MIN_VIDEO_WIDTH / VIDEO_ASPECT) + CONTROL_BAR_HEIGHT,
    score: -1,
  }
  let foundWithoutScroll = false

  for (let cols = 1; cols <= maxCols; cols++) {
    const rows = Math.ceil(count / cols)
    const availableWidth = width - GRID_GAP * (cols - 1)
    const cellWidth = availableWidth / cols
    if (cellWidth < MIN_VIDEO_WIDTH) continue

    const videoWidth = Math.floor(cellWidth)
    const videoHeight = videoWidth / VIDEO_ASPECT
    const tileWidth = videoWidth
    const tileHeight = Math.floor(videoHeight + CONTROL_BAR_HEIGHT)
    const requiredHeight = rows * tileHeight + GRID_GAP * (rows - 1)
    const score = tileWidth * videoHeight

    if (requiredHeight <= height && score > best.score) {
      foundWithoutScroll = true
      best = { cols, rows, tileWidth, tileHeight, score }
    }
  }

  if (!foundWithoutScroll) {
    const cols = maxCols
    const rows = Math.ceil(count / cols)
    const availableWidth = width - GRID_GAP * (cols - 1)
    const tileWidth = Math.max(MIN_VIDEO_WIDTH, Math.floor(availableWidth / cols))
    const tileHeight = Math.floor(tileWidth / VIDEO_ASPECT + CONTROL_BAR_HEIGHT)
    return { cols, rows, tileWidth, tileHeight, shouldScroll: true }
  }

  return {
    cols: best.cols,
    rows: best.rows,
    tileWidth: best.tileWidth,
    tileHeight: best.tileHeight,
    shouldScroll: false,
  }
})

const gridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${gridLayout.value.cols}, ${gridLayout.value.tileWidth}px)`,
  gridAutoRows: `${gridLayout.value.tileHeight}px`,
  gap: `${GRID_GAP}px`,
  justifyContent: 'center',
  alignContent: gridLayout.value.shouldScroll ? 'start' : 'center',
}))

function updateGridContainerSize() {
  if (!gridContainerRef.value) return
  const rect = gridContainerRef.value.getBoundingClientRect()
  containerSize.value = {
    width: Math.max(0, Math.floor(rect.width)),
    height: Math.max(0, Math.floor(rect.height)),
  }
}

async function refresh() {
  loading.value = true
  try {
    const [streamList] = await Promise.all([api.getStreams(), fetchRecordingStatus()])
    streams.value = streamList
    refreshKey.value++
    await nextTick()
    updateGridContainerSize()
  } catch (e: any) {
    showDeleteError(e?.message || 'Failed to refresh streams')
  } finally {
    loading.value = false
  }
}

async function fetchRecordingStatus() {
  try {
    const res = await api.getRecordingStatus()
    recordingState.value = parseRecordingRuntimeState(res)

    // If successful, ensure we are online
    if (isServerOffline.value) {
      isServerOffline.value = false
      stopConnectionCheck()
    }
  } catch (e) {
    // Go offline if we are not already
    if (!isServerOffline.value) {
      isServerOffline.value = true
      stopPolling()
      startConnectionCheck()
    }
  }
}

function startPolling() {
  stopPolling()
  fetchRecordingStatus() // Run once immediately
  pollInterval = setInterval(fetchRecordingStatus, 1000)
}

function stopPolling() {
  if (pollInterval) {
    clearInterval(pollInterval)
    pollInterval = undefined
  }
}

function startConnectionCheck() {
  if (connectionCheckInterval) clearInterval(connectionCheckInterval)
  checkConnection()
  connectionCheckInterval = setInterval(checkConnection, 3000)
}

function stopConnectionCheck() {
  if (connectionCheckInterval) {
    clearInterval(connectionCheckInterval)
    connectionCheckInterval = undefined
  }
}

async function checkConnection() {
  reconnecting.value = true
  try {
    // Try to fetch server info with a short timeout
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 2000)

    await fetch(apiURL(), { signal: controller.signal })
    clearTimeout(timeoutId)

    // Online
    isServerOffline.value = false
    stopConnectionCheck()
    startPolling()
    refresh()
  } catch (e) {
    // Still offline
  } finally {
    reconnecting.value = false
  }
}

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

async function toggleRecord(stream: string) {
  try {
    const status = getStreamStatus(stream)
    if (status === 'recording') {
      await api.stopRecording(stream)
    } else {
      // If idle or stopped, start recording
      await api.startRecording(stream)
    }
    // Refresh status immediately
    await fetchRecordingStatus()
  } catch (e: any) {
    showDeleteError(e?.message || 'Failed to toggle recording')
  }
}

function openLinks(stream: string) {
  selectedStream.value = stream
  showLinksModal.value = true
}

function openRecordConfig(stream: string) {
  selectedStreamForRecord.value = stream
  showRecordModal.value = true
}

function removeStream(stream: string) {
  pendingDeleteStream.value = stream
  showDeleteDialog.value = true
}

async function confirmRemoveStream() {
  if (!pendingDeleteStream.value) return
  try {
    await api.deleteStream(pendingDeleteStream.value)
    showDeleteDialog.value = false
    pendingDeleteStream.value = ''
    await refresh()
  } catch (e: any) {
    showDeleteError(e?.message || 'Failed to delete stream')
  }
}

function showDeleteError(message: string) {
  deleteErrorMessage.value = message
  if (deleteErrorTimer) {
    clearTimeout(deleteErrorTimer)
  }
  deleteErrorTimer = window.setTimeout(() => {
    deleteErrorMessage.value = ''
  }, 3500)
}

onMounted(() => {
  refresh()
  if (gridContainerRef.value) {
    gridResizeObserver = new ResizeObserver(() => updateGridContainerSize())
    gridResizeObserver.observe(gridContainerRef.value)
  }
  // Start polling
  startPolling()
})

onUnmounted(() => {
  if (gridResizeObserver) {
    gridResizeObserver.disconnect()
    gridResizeObserver = undefined
  }
  if (deleteErrorTimer) {
    clearTimeout(deleteErrorTimer)
    deleteErrorTimer = undefined
  }
  stopPolling()
  stopConnectionCheck()
})

watch(gridContainerRef, (el, prevEl) => {
  if (gridResizeObserver) {
    gridResizeObserver.disconnect()
    gridResizeObserver = undefined
  }
  if (prevEl && prevEl === el) return
  if (el) {
    gridResizeObserver = new ResizeObserver(() => updateGridContainerSize())
    gridResizeObserver.observe(el)
    updateGridContainerSize()
  }
})
</script>
