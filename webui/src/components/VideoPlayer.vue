<template>
  <div class="relative w-full h-full min-h-0 bg-black overflow-hidden rounded-lg shadow-lg group">
    <video-rtc
      ref="player"
      class="w-full h-full block"
      :mode="mode"
      :media="media"
      :background="background"
    ></video-rtc>

    <!-- Overlay for stream name/status -->
    <div
      class="absolute top-0 left-0 right-0 p-2 bg-linear-to-b from-black/70 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none z-10"
    >
      <div class="flex justify-between items-center text-white text-sm">
        <span class="font-medium truncate">{{ title || src }}</span>
        <div class="flex gap-2">
          <span v-if="status" class="px-1.5 py-0.5 rounded bg-blue-600/80 text-xs">{{
            status
          }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch, onBeforeUnmount } from 'vue'
import { websocketURL } from '@/lib/base-url'

const props = defineProps({
  src: {
    type: String,
    required: true,
  },
  title: {
    type: String,
    default: '',
  },
  mode: {
    type: String,
    default: 'webrtc,mse,hls,mjpeg',
  },
  media: {
    type: String,
    default: 'video,audio',
  },
  background: {
    type: Boolean,
    default: false,
  },
})

const player = ref<HTMLElement | null>(null)
const status = ref('')

onMounted(() => {
  if (player.value) {
    const el = player.value as any
    el.mode = props.mode
    el.media = props.media
    el.background = props.background

    // Set src last to trigger connection if needed, though usually bound by Vue
    // But since it's a custom element property, better to be explicit if reactivity is tricky
    if (props.src) {
      el.src = getStreamUrl(props.src)
    }

    // Add event listeners for status updates if VideoRTC emits events or we can hook into it
    // VideoRTC doesn't emit custom events for status changes in the original code,
    // but we can maybe observe it or just let it be.
    // The original video-stream.js extended it to add status.
    // Here we can perhaps use a MutationObserver or just trust the video element events?
  }
})

watch(
  () => props.src,
  (newVal) => {
    if (player.value) {
      if (newVal) {
        ;(player.value as any).src = getStreamUrl(newVal)
      } else {
        // If src is empty, we might want to stop the player
        // But VideoRTC doesn't have a clear stop method exposed directly that clears everything
        // Setting src to empty string might trigger disconnection logic if handled
        // (player.value as any).src = ''
      }
    }
  },
)

function getStreamUrl(src: string) {
  if (!src) return ''
  if (src.includes('://') || src.startsWith('/')) return src
  // It's a stream name
  const url = new URL(websocketURL('api/ws'))
  url.searchParams.set('src', src)
  return url.toString()
}

onBeforeUnmount(() => {
  // Cleanup if needed
})
</script>

<style scoped>
video-rtc {
  display: block;
  width: 100%;
  height: 100%;
}

:deep(video-rtc > video) {
  object-fit: contain;
}
</style>
