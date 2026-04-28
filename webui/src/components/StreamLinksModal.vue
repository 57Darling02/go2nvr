<template>
  <Dialog :open="props.open" @update:open="onOpenChange">
    <DialogContent
      class="h-[90vh] sm:max-w-3xl p-0 bg-gray-900 border-gray-700 text-white flex flex-col overflow-hidden"
      :show-close-button="false"
    >
      <DialogHeader class="px-4 pt-4 sm:px-6 sm:pt-6 shrink-0">
        <div class="flex items-start justify-between gap-3">
          <DialogTitle class="text-lg sm:text-xl text-white min-w-0">
            Stream Links: <span class="text-blue-400 break-all">{{ stream }}</span>
          </DialogTitle>
          <Button
            size="icon-sm"
            variant="ghost"
            class="text-gray-400 hover:text-white hover:bg-gray-800"
            @click="closeModal"
          >
            <X class="w-4 h-4" />
          </Button>
        </div>
      </DialogHeader>

      <div class="flex-1 min-h-0 px-4 pb-4 sm:px-6 sm:pb-6 overflow-hidden">
        <div v-if="loading" class="h-full py-8 flex items-center justify-center text-gray-400">
          <Loader2 class="w-4 h-4 mr-2 animate-spin" />
          Loading info...
        </div>

        <ScrollArea v-else class="h-full">
          <div class="space-y-6 pr-3">
            <section>
              <h3 class="text-sm font-bold text-gray-500 uppercase tracking-wider mb-2">
                Protocols
              </h3>
              <div class="space-y-2">
                <div v-for="(link, index) in links" :key="index" class="group">
                  <div
                    class="flex items-start sm:items-center justify-between gap-2 p-3 bg-gray-800 rounded hover:bg-gray-700 transition-colors"
                  >
                    <div class="flex-1 min-w-0 mr-1 sm:mr-4">
                      <div class="text-sm font-medium text-white truncate">{{ link.label }}</div>
                      <div class="text-xs text-gray-400 truncate font-mono mt-1">
                        {{ link.url }}
                      </div>
                    </div>
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      class="text-gray-400 hover:text-white hover:bg-gray-600"
                      :class="copiedUrl === link.url && 'text-green-500 hover:text-green-400'"
                      @click="copy(link.url)"
                    >
                      <Check v-if="copiedUrl === link.url" class="w-4 h-4" />
                      <Copy v-else class="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              </div>
            </section>

            <section>
              <h3 class="text-sm font-bold text-gray-500 uppercase tracking-wider mb-2">
                FFplay Command
              </h3>
              <div
                class="bg-gray-950 p-4 rounded font-mono text-xs text-gray-300 overflow-x-auto whitespace-nowrap border border-gray-800"
              >
                {{ ffplayCmd }}
              </div>
            </section>
          </div>
        </ScrollArea>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Check, Copy, Loader2, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import * as api from '@/services/api'

const props = withDefaults(defineProps<{ stream: string; open?: boolean }>(), {
  open: false,
})
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:open', value: boolean): void
}>()

const loading = ref(true)
const rtspHost = ref('')
const copiedUrl = ref('')

function onOpenChange(value: boolean) {
  emit('update:open', value)
  if (!value) {
    emit('close')
  }
}

function closeModal() {
  onOpenChange(false)
}

const links = computed(() => {
  const s = props.stream
  const host = window.location.hostname
  const protocol = window.location.protocol
  const port = window.location.port ? `:${window.location.port}` : ''
  const baseUrl = `${protocol}//${host}${port}`
  const rtsp = rtspHost.value || `${host}:8554`

  const getUrl = (proto: 'ws' | 'hls' | 'mp4' | 'mjpeg' | 'jpeg' | 'info', opts: any = {}) =>
    `${baseUrl}${api.getStreamUrl(s, proto, opts)}`

  return [
    { label: 'RTSP (Video/Audio)', url: `rtsp://${rtsp}/${s}` },
    { label: 'RTSP (MP4 Recording)', url: `rtsp://${rtsp}/${s}?mp4` },
    { label: 'HLS (fMP4)', url: getUrl('hls', { mp4: '' }) },
    { label: 'HLS (TS)', url: getUrl('hls') },
    { label: 'MSE / MP4 (WebSocket)', url: getUrl('ws') },
    { label: 'MP4 Stream', url: getUrl('mp4') },
    { label: 'MJPEG Stream', url: getUrl('mjpeg') },
    { label: 'Snapshot (JPEG)', url: getUrl('jpeg') },
    { label: 'Stream Info (JSON)', url: getUrl('info') },
  ]
})

const ffplayCmd = computed(() => {
  const rtsp = rtspHost.value || `${window.location.hostname}:8554`
  return `ffplay -fflags nobuffer -flags low_delay -rtsp_transport tcp "rtsp://${rtsp}/${props.stream}"`
})

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    copiedUrl.value = text
    setTimeout(() => {
      if (copiedUrl.value === text) {
        copiedUrl.value = ''
      }
    }, 2000)
  } catch (e) {
    console.error('Failed to copy', e)
  }
}

onMounted(async () => {
  try {
    const info = await api.getServerInfo()
    if (info?.rtsp?.listen) {
      const port = info.rtsp.listen.split(':').pop()
      if (port) {
        rtspHost.value = `${window.location.hostname}:${port}`
      }
    }
  } catch (e) {
    console.error('Failed to fetch server info:', e)
  }
  loading.value = false
})
</script>
