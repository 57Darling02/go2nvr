<template>
  <div
    :class="
      cn(
        'flex h-full min-h-0 w-full flex-col overflow-hidden rounded-lg border border-gray-800 bg-gray-950 shadow-sm',
        props.class,
      )
    "
  >
    <div class="relative min-h-0 flex-1 overflow-hidden bg-black">
      <div class="absolute right-2 top-2 z-10 w-28">
        <Select v-model="playbackMode">
          <SelectTrigger
            class="h-8 border-gray-700 bg-gray-900/90 text-xs text-white focus-visible:border-blue-500 focus-visible:ring-blue-500/40"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent class="border-gray-700 bg-gray-900 text-white">
            <SelectItem value="hls" class="text-gray-200 focus:bg-gray-800 focus:text-white">
              HLS
            </SelectItem>
            <SelectItem value="webrtc" class="text-gray-200 focus:bg-gray-800 focus:text-white">
              WebRTC
            </SelectItem>
          </SelectContent>
        </Select>
      </div>

      <VideoPlayer
        v-if="props.src"
        :key="playerKey"
        :src="props.src"
        :title="props.title || props.src"
        :mode="playerMode"
        class="h-full w-full"
      />
      <div v-else class="flex h-full items-center justify-center text-sm text-gray-500">
        {{ props.emptyText }}
      </div>
    </div>

    <div class="shrink-0 border-t border-gray-800 bg-gray-900 p-3">
      <div
        v-if="props.showDashboardTools"
        class="flex items-center justify-between gap-2 flex-wrap"
      >
        <div class="flex items-center gap-2 max-w-full shrink-0 pr-1 sm:pr-2">
          <div class="truncate font-medium text-sm text-gray-300" :title="props.title || props.src">
            {{ props.title || props.src || 'No stream selected' }}
          </div>
          <span
            v-if="props.triggerLabel"
            class="shrink-0 text-[10px] px-1.5 py-0.5 rounded border border-blue-500/50 text-blue-300 bg-blue-500/10"
          >
            {{ props.triggerLabel }}
          </span>
        </div>
        <div class="flex gap-1.5 sm:gap-2 shrink-0 ml-auto">
          <button
            type="button"
            @click="emit('open-links')"
            class="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-white transition-colors focus:outline-none focus:ring-1 focus:ring-gray-700"
            title="Stream Links"
          >
            <LinkIcon class="w-4 h-4" />
          </button>

          <button
            type="button"
            @click="emit('toggle-record')"
            class="p-1.5 rounded hover:bg-gray-800 transition-colors focus:outline-none focus:ring-1 focus:ring-gray-700"
            :title="props.recordToggleTitle || 'Toggle Recording'"
          >
            <div
              :class="[
                'w-3 h-3 rounded-full transition-colors',
                props.statusDotClass || 'bg-gray-600',
              ]"
            />
          </button>

          <button
            type="button"
            @click="emit('open-record-settings')"
            class="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-white transition-colors focus:outline-none focus:ring-1 focus:ring-gray-700"
            title="Recording Settings"
          >
            <Sliders class="w-4 h-4" />
          </button>

          <router-link
            :to="props.recordingsTo || '/recordings'"
            class="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-white transition-colors focus:outline-none focus:ring-1 focus:ring-gray-700"
            title="View Recordings"
          >
            <History class="w-4 h-4" />
          </router-link>

          <button
            type="button"
            @click="emit('delete-stream')"
            class="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-red-400 transition-colors focus:outline-none focus:ring-1 focus:ring-gray-700"
            title="Delete Stream"
          >
            <Trash2 class="w-4 h-4" />
          </button>
        </div>
      </div>
      <slot v-else name="footer">
        <div class="truncate text-sm font-medium text-gray-300">
          {{ props.title || props.src || 'No stream selected' }}
        </div>
      </slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { HTMLAttributes } from 'vue'
import { History, Link as LinkIcon, Sliders, Trash2 } from 'lucide-vue-next'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'
import VideoPlayer from '@/components/VideoPlayer.vue'

type PlaybackMode = 'hls' | 'webrtc'

const props = withDefaults(
  defineProps<{
    src?: string
    title?: string
    emptyText?: string
    initialMode?: PlaybackMode
    modeStorageKey?: string
    showDashboardTools?: boolean
    triggerLabel?: string
    statusDotClass?: string
    recordToggleTitle?: string
    recordingsTo?: string
    class?: HTMLAttributes['class']
  }>(),
  {
    emptyText: 'Select a stream to start preview',
    initialMode: 'hls',
    showDashboardTools: false,
  },
)

const emit = defineEmits<{
  (e: 'mode-change', value: PlaybackMode): void
  (e: 'open-links'): void
  (e: 'toggle-record'): void
  (e: 'open-record-settings'): void
  (e: 'delete-stream'): void
}>()

const playbackMode = ref<PlaybackMode>(props.initialMode)

const playerMode = computed(() => {
  return playbackMode.value === 'webrtc' ? 'webrtc,mse,hls,mjpeg' : 'hls,mse,mjpeg'
})

const playerKey = computed(() => {
  return `${props.src || 'empty'}:${playbackMode.value}`
})

onMounted(() => {
  if (!props.modeStorageKey) return
  const savedMode = localStorage.getItem(props.modeStorageKey)
  if (savedMode === 'hls' || savedMode === 'webrtc') {
    playbackMode.value = savedMode
  }
})

watch(playbackMode, (mode) => {
  if (props.modeStorageKey) {
    localStorage.setItem(props.modeStorageKey, mode)
  }
  emit('mode-change', mode)
})
</script>
