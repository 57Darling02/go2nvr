<template>
  <div class="flex h-full overflow-hidden bg-black">
    <div
      class="w-full md:w-80 lg:w-96 bg-gray-950 border-r border-gray-800 flex flex-col shrink-0 min-h-0"
      :class="{ 'hidden md:flex': currentFile }"
    >
      <div class="p-3 sm:p-4 border-b border-gray-800 flex items-center justify-between gap-2">
        <span class="font-semibold text-gray-300">Recordings</span>
        <Button
          variant="ghost"
          size="icon-sm"
          class="text-gray-400 hover:text-white"
          :disabled="loading"
          @click="refreshCurrent"
        >
          <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        </Button>
      </div>

      <div
        v-if="currentStream || currentDate"
        class="flex items-center px-3 sm:px-4 py-2.5 bg-gray-900 border-b border-gray-800 text-sm"
      >
        <Button
          variant="link"
          class="h-auto p-0 text-blue-400 hover:text-blue-300"
          @click="navigateUp"
        >
          <ChevronLeft class="w-4 h-4 mr-1" />
          {{ currentDate ? 'Back to Dates' : 'Back to Streams' }}
        </Button>
      </div>

      <div
        v-if="currentStream"
        class="px-3 sm:px-4 py-2 bg-gray-900/50 text-xs font-bold text-gray-500 uppercase tracking-wider border-b border-gray-800 truncate"
      >
        {{ currentStream.name }} {{ currentDate ? `/ ${currentDate.name}` : '' }}
      </div>

      <div class="p-3 sm:p-4 pb-0" v-if="errorMessage">
        <Alert
          variant="destructive"
          class="border-red-800 bg-red-900/40 text-red-200 [&_p]:text-red-200"
        >
          <AlertDescription>{{ errorMessage }}</AlertDescription>
        </Alert>
      </div>

      <div class="flex-1 min-h-0">
        <ScrollArea class="h-full">
          <div v-if="!currentStream">
            <button
              v-for="stream in streams"
              :key="stream.path"
              @click="selectStream(stream)"
              type="button"
              class="w-full px-3 sm:px-4 py-3 hover:bg-gray-900 text-left flex justify-between items-center group transition-colors border-b border-gray-900"
            >
              <div class="flex items-center min-w-0">
                <Video class="w-4 h-4 text-gray-500 mr-3" />
                <span class="text-gray-300 group-hover:text-white truncate">{{ stream.name }}</span>
              </div>
              <ChevronRight class="w-4 h-4 text-gray-600 group-hover:text-gray-400 shrink-0" />
            </button>
            <div
              v-if="!loading && streams.length === 0"
              class="p-6 text-center text-gray-500 text-sm"
            >
              No streams found
            </div>
          </div>

          <div v-else-if="!currentDate">
            <button
              v-for="date in dates"
              :key="date.path"
              @click="selectDate(date)"
              type="button"
              class="w-full px-3 sm:px-4 py-3 hover:bg-gray-900 text-left flex justify-between items-center group transition-colors border-b border-gray-900"
            >
              <div class="flex items-center min-w-0">
                <Calendar class="w-4 h-4 text-gray-500 mr-3" />
                <span class="text-gray-300 group-hover:text-white truncate">{{ date.name }}</span>
              </div>
              <ChevronRight class="w-4 h-4 text-gray-600 group-hover:text-gray-400 shrink-0" />
            </button>
            <div
              v-if="!loading && dates.length === 0"
              class="p-6 text-center text-gray-500 text-sm"
            >
              No dates found
            </div>
          </div>

          <div v-else>
            <button
              v-for="file in files"
              :key="file.path"
              @click="playFile(file)"
              type="button"
              class="w-full px-3 sm:px-4 py-3 hover:bg-gray-900 text-left transition-colors border-l-2 border-b border-gray-900"
              :class="
                currentFile?.path === file.path
                  ? 'bg-gray-900 border-l-blue-500 text-white'
                  : 'border-l-transparent text-gray-400'
              "
            >
              <div class="flex justify-between items-center">
                <div class="flex items-center truncate mr-2 min-w-0">
                  <div
                    class="relative w-16 h-9 bg-gray-800 mr-3 shrink-0 rounded overflow-hidden flex items-center justify-center"
                  >
                    <img
                      v-if="file.thumbUrl"
                      :src="file.thumbUrl"
                      class="w-full h-full object-cover"
                      loading="lazy"
                      @error="file.thumbUrl = null"
                    />
                    <FileVideo
                      v-else
                      class="w-5 h-5"
                      :class="currentFile?.path === file.path ? 'text-blue-400' : 'text-gray-600'"
                    />
                  </div>
                  <span class="text-sm font-medium truncate">{{ file.name }}</span>
                </div>
                <span class="text-xs text-gray-600 shrink-0">{{ formatSize(file.size) }}</span>
              </div>
            </button>
            <div
              v-if="!loading && files.length === 0"
              class="p-6 text-center text-gray-500 text-sm"
            >
              No files found
            </div>
          </div>
        </ScrollArea>
      </div>
    </div>

    <div
      class="flex-1 flex flex-col bg-black relative min-w-0"
      :class="{ 'hidden md:flex': !currentFile }"
    >
      <div v-if="currentFile" class="flex-1 flex flex-col h-full min-h-0">
        <div class="md:hidden px-3 py-2 bg-gray-900 border-b border-gray-800 flex items-center">
          <Button
            variant="link"
            class="h-auto p-0 text-blue-400 hover:text-blue-300"
            @click="closePlayer"
          >
            <ChevronLeft class="w-4 h-4 mr-1" /> Back to List
          </Button>
        </div>

        <div class="flex-1 relative bg-black flex items-center justify-center overflow-hidden">
          <video
            ref="videoEl"
            controls
            autoplay
            class="max-w-full max-h-full w-full h-full object-contain focus:outline-none"
            :src="videoUrl"
          ></video>
        </div>
        <div
          class="min-h-16 bg-gray-900 border-t border-gray-800 flex flex-col gap-2 px-3 py-2 sm:flex-row sm:items-center sm:justify-between sm:px-6 shrink-0"
        >
          <div class="truncate min-w-0 w-full sm:w-auto sm:mr-4">
            <h2
              class="text-white font-medium text-sm md:text-base truncate"
              :title="currentFile.name"
            >
              {{ currentFile.name }}
            </h2>
            <p class="text-gray-500 text-xs truncate">{{ currentStream?.name }} / {{ currentDate?.name }}</p>
          </div>
          <div class="flex gap-2 sm:gap-3 shrink-0 flex-wrap">
            <Button
              as-child
              variant="outline"
              class="border-gray-700 bg-gray-800 hover:bg-gray-700 text-gray-300 hover:text-white text-xs sm:text-sm"
            >
              <a :href="videoUrl" download> <Download class="w-4 h-4 mr-2" /> Download </a>
            </Button>
            <Button
              @click="requestDeleteFile(currentFile)"
              variant="outline"
              class="border-red-700/60 bg-red-900/20 hover:bg-red-900/40 text-red-400 hover:text-red-300 text-xs sm:text-sm"
            >
              <Trash2 class="w-4 h-4 mr-2" /> Delete
            </Button>
          </div>
        </div>
      </div>
      <div
        v-else
        class="flex-1 flex items-center justify-center text-gray-700 flex-col px-4 text-center"
      >
        <Film class="w-16 h-16 sm:w-20 sm:h-20 mb-6 opacity-20" />
        <p class="text-base sm:text-lg font-medium">Select a recording to play</p>
        <p class="text-xs sm:text-sm mt-2 opacity-60">Browse streams and dates on the left</p>
      </div>
    </div>

    <AlertDialog :open="deleteDialogOpen" @update:open="deleteDialogOpen = $event">
      <AlertDialogContent class="bg-gray-900 border-gray-700 text-white">
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Recording</AlertDialogTitle>
          <AlertDialogDescription class="text-gray-300 break-all">
            Are you sure you want to delete {{ pendingDeleteFile?.name }}?
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel
            class="border-gray-600 bg-gray-800 text-gray-200 hover:bg-gray-700 hover:text-white"
          >
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction
            class="bg-red-600 text-white hover:bg-red-500"
            @click="confirmDeleteFile"
          >
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  ChevronRight,
  ChevronLeft,
  Film,
  Video,
  Calendar,
  FileVideo,
  Download,
  Trash2,
  RefreshCw,
} from 'lucide-vue-next'
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
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import * as api from '@/services/api'

type RecordingFileItem = api.Recording & {
  thumbUrl: string | null
}

const route = useRoute()

const streams = ref<api.Recording[]>([])
const dates = ref<api.Recording[]>([])
const files = ref<RecordingFileItem[]>([])

const loading = ref(false)
const errorMessage = ref('')

const currentStream = ref<api.Recording | null>(null)
const currentDate = ref<api.Recording | null>(null)
const currentFile = ref<RecordingFileItem | null>(null)
const pendingDeleteFile = ref<RecordingFileItem | null>(null)
const deleteDialogOpen = ref(false)
const videoUrl = ref('')
const videoEl = ref<HTMLVideoElement | null>(null)

function formatSize(bytes?: number) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

async function loadStreams(selectSrc?: string) {
  loading.value = true
  errorMessage.value = ''
  try {
    const res = await api.getRecordings('.')
    if (Array.isArray(res)) {
      streams.value = res.filter((item) => !item.is_file).sort((a, b) => a.name.localeCompare(b.name))
      if (selectSrc) {
        const stream = streams.value.find((item) => item.name === selectSrc)
        if (stream) await selectStream(stream)
      }
    }
  } catch (e: any) {
    errorMessage.value = e?.message || 'Failed to load streams'
  } finally {
    loading.value = false
  }
}

async function selectStream(stream: api.Recording) {
  currentStream.value = stream
  currentDate.value = null
  files.value = []
  currentFile.value = null
  loading.value = true
  errorMessage.value = ''
  try {
    const res = await api.getRecordings(stream.path)
    if (Array.isArray(res)) {
      dates.value = res
        .filter((item) => !item.is_file)
        .sort((a, b) => b.name.localeCompare(a.name))
    }
  } catch (e: any) {
    errorMessage.value = e?.message || 'Failed to load dates'
  } finally {
    loading.value = false
  }
}

async function selectDate(date: api.Recording) {
  currentDate.value = date
  loading.value = true
  errorMessage.value = ''
  try {
    const res = await api.getRecordings(date.path)
    if (Array.isArray(res)) {
      files.value = res
        .filter((item) => item.is_file && item.name.endsWith('.mp4'))
        .sort((a, b) => b.name.localeCompare(a.name))
        .map((item) => {
          const thumbPath = item.path.replace(/\.mp4$/i, '.thumb')
          return {
            ...item,
            thumbUrl: api.getRecordingFileUrl(thumbPath),
          } as RecordingFileItem
        })
    }
  } catch (e: any) {
    errorMessage.value = e?.message || 'Failed to load files'
  } finally {
    loading.value = false
  }
}

function playFile(file: RecordingFileItem) {
  currentFile.value = file
  videoUrl.value = api.getRecordingFileUrl(file.path)
}

function releaseVideoPlayback() {
  const el = videoEl.value
  if (!el) return
  try {
    el.pause()
    el.removeAttribute('src')
    el.load()
  } catch (_) {}
}

function closePlayer() {
  releaseVideoPlayback()
  currentFile.value = null
  videoUrl.value = ''
}

function requestDeleteFile(file: RecordingFileItem | null) {
  if (!file) return
  pendingDeleteFile.value = file
  deleteDialogOpen.value = true
}

async function confirmDeleteFile() {
  const target = pendingDeleteFile.value
  deleteDialogOpen.value = false
  if (!target) return
  try {
    if (currentFile.value?.path === target.path) {
      releaseVideoPlayback()
      currentFile.value = null
      videoUrl.value = ''
      await new Promise((resolve) => setTimeout(resolve, 80))
    }
    const filePath = target.path
    await api.deleteRecordingFile(filePath)
    const thumbPath = filePath.replace(/\.mp4$/i, '.thumb')
    if (thumbPath !== filePath) {
      try {
        await api.deleteRecordingFile(thumbPath)
      } catch (e: any) {
        const message = String(e?.message || '')
        if (!/404|not\s*found/i.test(message)) {
          throw e
        }
      }
    }
    if (currentDate.value) await selectDate(currentDate.value)
    currentFile.value = null
    videoUrl.value = ''
  } catch (e: any) {
    errorMessage.value = e?.message || `Failed to delete ${target.name}`
  } finally {
    pendingDeleteFile.value = null
  }
}

function navigateUp() {
  if (currentDate.value) {
    currentDate.value = null
    files.value = []
    currentFile.value = null
  } else if (currentStream.value) {
    currentStream.value = null
    dates.value = []
    loadStreams()
  }
}

function refreshCurrent() {
  if (currentDate.value) {
    if (currentDate.value) selectDate(currentDate.value)
    return
  }
  if (currentStream.value) {
    if (currentStream.value) selectStream(currentStream.value)
    return
  }
  loadStreams()
}

onMounted(() => {
  loadStreams(typeof route.query.src === 'string' ? route.query.src : undefined)
})

watch(
  () => route.query.src,
  (newSrc) => {
    if (typeof newSrc === 'string' && newSrc !== currentStream.value?.name) {
      loadStreams(newSrc)
    }
  },
)
</script>
