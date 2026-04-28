<template>
  <Dialog :open="props.open" @update:open="onOpenChange">
    <DialogContent
      class="max-h-[92vh] sm:max-w-xl p-0 bg-gray-900 border-gray-700 text-white overflow-hidden"
      :show-close-button="false"
    >
      <DialogHeader class="px-4 pt-4 sm:px-6 sm:pt-6 shrink-0">
        <DialogTitle class="text-lg sm:text-xl text-white">Add Stream</DialogTitle>
        <DialogDescription class="text-gray-400">
          Configure source and optionally discover ONVIF devices.
        </DialogDescription>
      </DialogHeader>

      <form
        @submit.prevent="submit"
        class="px-4 pb-4 sm:px-6 sm:pb-6 space-y-5 overflow-y-auto max-h-[calc(92vh-5.5rem)]"
      >
        <div class="space-y-2">
          <Label for="stream-name" class="text-gray-300">Stream Name</Label>
          <Input
            id="stream-name"
            v-model="name"
            type="text"
            placeholder="camera1"
            required
            class="bg-gray-800 border-gray-600 text-white placeholder:text-gray-500 focus-visible:ring-blue-500/40 focus-visible:border-blue-500"
          />
        </div>

        <div class="space-y-2">
          <Label for="stream-src" class="text-gray-300">Source URL</Label>
          <Input
            id="stream-src"
            v-model="src"
            type="text"
            placeholder="rtsp://..."
            required
            class="bg-gray-800 border-gray-600 text-white placeholder:text-gray-500 focus-visible:ring-blue-500/40 focus-visible:border-blue-500"
          />
          <p class="text-xs text-gray-500">Supports RTSP, RTMP, HTTP, FFmpeg, ONVIF, etc.</p>
        </div>

        <div class="space-y-3 rounded-lg border border-gray-700 bg-gray-950/60 p-3">
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <h3 class="text-sm font-semibold text-gray-200">ONVIF Discovery</h3>
            <Button
              type="button"
              size="sm"
              class="h-8 bg-indigo-600 hover:bg-indigo-500 text-white"
              :disabled="discovering"
              @click="discoverOnvif"
            >
              {{ discovering ? 'Discovering...' : 'Auto Discover' }}
            </Button>
          </div>

          <div class="flex gap-2 flex-col sm:flex-row">
            <Input
              v-model="onvifTestSrc"
              type="text"
              placeholder="onvif://user:pass@192.168.1.123:80"
              class="flex-1 bg-gray-800 border-gray-600 text-white placeholder:text-gray-500 focus-visible:ring-blue-500/40 focus-visible:border-blue-500"
            />
            <Button
              type="button"
              variant="secondary"
              class="w-full sm:w-auto bg-gray-700 hover:bg-gray-600 text-white dark:bg-gray-700 dark:hover:bg-gray-600"
              :disabled="testingOnvif || !onvifTestSrc.trim()"
              @click="testOnvifSource"
            >
              {{ testingOnvif ? 'Testing...' : 'Test' }}
            </Button>
          </div>

          <Alert
            v-if="discoverError"
            variant="destructive"
            class="border-red-800 bg-red-900/30 text-red-200 [&_p]:text-red-200"
          >
            <AlertDescription>{{ discoverError }}</AlertDescription>
          </Alert>

          <div v-if="onvifSources.length > 0" class="max-h-40 overflow-y-auto space-y-2 pr-1">
            <button
              v-for="item in onvifSources"
              :key="item.key"
              type="button"
              @click="applyDiscoveredSource(item)"
              class="w-full text-left px-3 py-2 rounded border border-gray-700 bg-gray-900 hover:bg-gray-800 transition-colors"
            >
              <div class="text-sm text-gray-100 truncate">{{ item.name || item.url }}</div>
              <div class="text-xs text-gray-400 truncate">{{ item.url }}</div>
            </button>
          </div>

          <p v-else class="text-xs text-gray-500">
            Click Auto Discover to scan ONVIF devices in current subnet.
          </p>
        </div>

        <Alert
          v-if="error"
          variant="destructive"
          class="border-red-800 bg-red-900/50 text-red-200 [&_p]:text-red-200"
        >
          <AlertDescription>{{ error }}</AlertDescription>
        </Alert>

        <DialogFooter class="gap-2 sm:gap-3">
          <Button
            type="button"
            variant="outline"
            class="border-gray-600 bg-gray-800 text-gray-200 hover:bg-gray-700 hover:text-white"
            @click="closeModal"
          >
            Cancel
          </Button>
          <Button
            type="submit"
            class="bg-blue-600 hover:bg-blue-500 text-white"
            :disabled="loading || !name.trim() || !src.trim()"
          >
            {{ loading ? 'Adding...' : 'Add Stream' }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import * as api from '@/services/api'

const props = withDefaults(defineProps<{ open?: boolean }>(), {
  open: false,
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'added'): void
  (e: 'update:open', value: boolean): void
}>()

const name = ref('')
const src = ref('')
const loading = ref(false)
const error = ref('')
const discovering = ref(false)
const testingOnvif = ref(false)
const discoverError = ref('')
const onvifTestSrc = ref('')
const onvifSources = ref<Array<{ key: string; name: string; url: string }>>([])

function onOpenChange(value: boolean) {
  emit('update:open', value)
  if (!value) {
    emit('close')
  }
}

function closeModal() {
  onOpenChange(false)
}

function parseOnvifSources(payload: any): Array<{ key: string; name: string; url: string }> {
  const rows = Array.isArray(payload?.sources) ? payload.sources : []
  return rows
    .map((row: any, idx: number) => {
      const url = String(row?.url || row?.source || '').trim()
      if (!url) return null
      const label = String(row?.name || row?.info || row?.id || `ONVIF-${idx + 1}`).trim()
      return {
        key: `${label}-${url}-${idx}`,
        name: label,
        url,
      }
    })
    .filter(Boolean) as Array<{ key: string; name: string; url: string }>
}

function suggestNameFromUrl(url: string) {
  const match = url.match(/@([0-9a-zA-Z.\-]+)(?::\d+)?/)
  if (!match) return ''
  const host = match[1] ?? ''
  if (!host) return ''
  return `cam_${host.replace(/[^a-zA-Z0-9_]/g, '_')}`
}

function applyDiscoveredSource(item: { name: string; url: string }) {
  src.value = item.url
  if (!name.value.trim()) {
    const suggested = suggestNameFromUrl(item.url)
    if (suggested) name.value = suggested
  }
}

async function discoverOnvif() {
  discovering.value = true
  discoverError.value = ''
  try {
    const data = await api.discover('onvif')
    onvifSources.value = parseOnvifSources(data)
    if (onvifSources.value.length === 0) {
      discoverError.value = 'No ONVIF sources found'
    }
  } catch (e: any) {
    discoverError.value = e?.message || 'Failed to discover ONVIF sources'
    onvifSources.value = []
  } finally {
    discovering.value = false
  }
}

async function testOnvifSource() {
  const value = onvifTestSrc.value.trim()
  if (!value) return
  testingOnvif.value = true
  discoverError.value = ''
  try {
    const data = await api.discover('onvif', { src: value })
    onvifSources.value = parseOnvifSources(data)
    const firstSource = onvifSources.value[0]
    if (firstSource) {
      applyDiscoveredSource(firstSource)
    } else {
      discoverError.value = 'No ONVIF sources found from test URL'
    }
  } catch (e: any) {
    discoverError.value = e?.message || 'Failed to test ONVIF source'
  } finally {
    testingOnvif.value = false
  }
}

async function submit() {
  if (!name.value || !src.value) return

  loading.value = true
  error.value = ''

  try {
    await api.addStream(name.value, src.value)
    emit('added')
    closeModal()
  } catch (e: any) {
    error.value = e.message || 'Failed to add stream'
  } finally {
    loading.value = false
  }
}
</script>
