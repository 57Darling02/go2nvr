<template>
  <div class="flex flex-col h-full bg-gray-900 text-white">
    <header
      class="flex flex-col gap-3 px-3 py-3 bg-gray-900 border-b border-gray-800 sm:flex-row sm:items-center sm:justify-between sm:px-6 sm:py-4"
    >
      <div class="flex items-center gap-2 sm:gap-3 min-w-0">
        <h1 class="text-base sm:text-xl font-semibold text-white truncate">System</h1>
        <Button
          variant="ghost"
          size="icon"
          class="text-gray-400 hover:text-white"
          @click="refreshAll"
          title="Reload System Info & Logs"
        >
          <RefreshCw class="w-5 h-5" :class="{ 'animate-spin': loadingAll }" />
        </Button>
      </div>
    </header>

    <div class="flex-1 overflow-y-auto p-3 sm:p-6 space-y-4 sm:space-y-6" :key="refreshKey">
      <Alert
        v-if="notice"
        :variant="noticeType === 'error' ? 'destructive' : 'default'"
        :class="
          noticeType === 'error'
            ? 'border-red-800 bg-red-900/40 text-red-200 [&_p]:text-red-200'
            : 'border-green-800 bg-green-900/30 text-green-200 [&_p]:text-green-200'
        "
      >
        <AlertDescription>{{ notice }}</AlertDescription>
      </Alert>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 sm:gap-6">
        <section class="bg-gray-950 border border-gray-800 rounded-lg p-4 sm:p-6">
          <h2 class="text-lg font-medium mb-4 flex items-center">
            <Server class="w-5 h-5 mr-2 text-blue-500" />
            Server Control
          </h2>
          <div class="flex flex-col gap-3 xl:flex-row">
            <Button
              @click="openConfirm('restartServer')"
              variant="outline"
              class="w-full justify-start xl:flex-1 border-yellow-600/50 bg-yellow-600/10 text-yellow-400 hover:bg-yellow-600/20 hover:text-yellow-300"
            >
              <RotateCw class="w-4 h-4 mr-2" />
              Restart Server
            </Button>
            <Button
              @click="openConfirm('stopServer')"
              variant="outline"
              class="w-full justify-start xl:flex-1 border-red-600/50 bg-red-600/10 text-red-400 hover:bg-red-600/20 hover:text-red-300"
            >
              <Power class="w-4 h-4 mr-2" />
              Stop Server
            </Button>
          </div>
          <p class="mt-4 text-xs text-gray-500">
            Restarting will reload the configuration. Stopping requires manual restart of the
            service.
          </p>
        </section>

        <section class="bg-gray-950 border border-gray-800 rounded-lg p-4 sm:p-6">
          <h2 class="text-lg font-medium mb-4 flex items-center">
            <Info class="w-5 h-5 mr-2 text-blue-500" />
            Core Info
          </h2>
          <div class="space-y-2 text-sm">
            <div
              v-for="row in coreInfoRows"
              :key="row.label"
              class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-1 py-1 border-b border-gray-800"
            >
              <span class="text-gray-400">{{ row.label }}</span>
              <span
                class="font-mono text-xs sm:max-w-[60%] break-all sm:truncate"
                :title="row.value"
                >{{ row.value }}</span
              >
            </div>
            <div
              class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-1 py-1 pt-2"
            >
              <span class="text-gray-400">Debug</span>
              <Button
                variant="link"
                class="h-auto p-0 text-blue-400 hover:text-blue-300"
                @click="showStack"
              >
                View Stack Trace
              </Button>
            </div>
          </div>
        </section>
      </div>

      <section
        class="bg-gray-950 border border-gray-800 rounded-lg flex flex-col min-h-[24rem] sm:min-h-[30rem]"
      >
        <div
          class="flex items-center justify-between gap-3 flex-wrap p-3 sm:p-4 border-b border-gray-800 bg-gray-900/50"
        >
          <h2 class="text-lg font-medium flex items-center">
            <FileText class="w-5 h-5 mr-2 text-blue-500" />
            System Logs
          </h2>
          <div class="flex items-center gap-3 flex-wrap w-full sm:w-auto sm:justify-end">
            <label class="flex items-center text-xs text-gray-400 cursor-pointer">
              <input
                type="checkbox"
                v-model="autoRefresh"
                class="mr-2 rounded bg-gray-800 border-gray-700 text-blue-600 focus:ring-0"
              />
              Auto Refresh (5s)
            </label>
            <Button
              @click="openConfirm('clearLogs')"
              variant="outline"
              class="text-xs border-gray-700 bg-gray-800 text-gray-300 hover:bg-gray-700 hover:text-white"
            >
              Clear Logs
            </Button>
          </div>
        </div>

        <div class="flex-1 overflow-auto p-0 font-mono text-xs">
          <table class="w-full min-w-[520px] text-left border-collapse">
            <thead class="bg-gray-900 sticky top-0 z-10">
              <tr>
                <th class="p-2 border-b border-gray-800 w-32 font-medium text-gray-500">Time</th>
                <th class="p-2 border-b border-gray-800 w-20 font-medium text-gray-500">Level</th>
                <th class="p-2 border-b border-gray-800 font-medium text-gray-500">Message</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-800/50">
              <tr v-for="(log, i) in logs" :key="i" class="hover:bg-gray-900/30 transition-colors">
                <td class="p-2 text-gray-500 whitespace-nowrap">{{ formatTime(log.time) }}</td>
                <td class="p-2 font-bold" :class="getLevelClass(log.level)">{{ log.level }}</td>
                <td class="p-2 text-gray-300 break-all">
                  {{ log.message || log.msg || JSON.stringify(log) }}
                </td>
              </tr>
              <tr v-if="logs.length === 0">
                <td colspan="3" class="p-8 text-center text-gray-600">No logs available</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <Dialog :open="showStackModal" @update:open="showStackModal = $event">
      <DialogContent
        class="w-[calc(100vw-1.5rem)] sm:max-w-4xl h-[80vh] p-0 bg-gray-900 border-gray-700 text-white overflow-hidden"
        :show-close-button="false"
      >
        <DialogHeader class="px-4 py-3 border-b border-gray-800">
          <div class="flex justify-between items-center gap-3">
            <DialogTitle class="text-lg font-semibold text-white">Stack Trace</DialogTitle>
            <Button
              size="icon-sm"
              variant="ghost"
              class="text-gray-400 hover:text-white hover:bg-gray-800"
              @click="showStackModal = false"
            >
              <X class="w-4 h-4" />
            </Button>
          </div>
        </DialogHeader>
        <div
          class="flex-1 overflow-auto p-4 bg-black text-green-400 font-mono text-xs whitespace-pre"
        >
          <ScrollArea class="h-full w-full">
            <pre class="text-green-400 font-mono text-xs whitespace-pre">{{ stackTrace }}</pre>
            <ScrollBar orientation="horizontal" />
          </ScrollArea>
        </div>
      </DialogContent>
    </Dialog>

    <AlertDialog :open="confirmOpen" @update:open="confirmOpen = $event">
      <AlertDialogContent class="bg-gray-900 border-gray-700 text-white">
        <AlertDialogHeader>
          <AlertDialogTitle>{{ confirmMeta.title }}</AlertDialogTitle>
          <AlertDialogDescription class="text-gray-300">
            {{ confirmMeta.description }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel
            class="border-gray-600 bg-gray-800 text-gray-200 hover:bg-gray-700 hover:text-white"
          >
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction :class="confirmMeta.actionClass" @click="executeConfirmAction">
            {{ confirmMeta.actionLabel }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

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
          <Button
            v-else
            @click="checkConnection"
            class="w-full bg-blue-600 text-white hover:bg-blue-500"
          >
            <RefreshCw class="w-4 h-4 mr-2" />
            Retry Connection
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, watch } from 'vue'
import { Server, RotateCw, Power, FileText, RefreshCw, Info, X } from 'lucide-vue-next'
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
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ScrollArea, ScrollBar } from '@/components/ui/scroll-area'
import * as api from '@/services/api'

const version = ref('Loading...')
const schemes = ref<string[]>([])
const logs = ref<any[]>([])
const loadingLogs = ref(false)
const loadingAll = ref(false)
const refreshKey = ref(0)
const showStackModal = ref(false)
const stackTrace = ref('')
const autoRefresh = ref(true)
const notice = ref('')
const noticeType = ref<'success' | 'error'>('success')

const isServerOffline = ref(false)
const reconnecting = ref(false)
let connectionCheckInterval: number | undefined

let pollInterval: number | undefined
const confirmOpen = ref(false)
const confirmAction = ref<'clearLogs' | 'restartServer' | 'stopServer' | null>(null)

const coreInfoRows = computed(() => [
  { label: 'Version', value: version.value || 'Unknown' },
  { label: 'Supported Schemes', value: schemes.value.join(', ') || 'Loading...' },
])

const confirmMeta = computed(() => {
  if (confirmAction.value === 'clearLogs') {
    return {
      title: 'Clear Logs',
      description: 'This action will permanently remove all current logs.',
      actionLabel: 'Clear Logs',
      actionClass: 'bg-red-600 text-white hover:bg-red-500',
    }
  }
  if (confirmAction.value === 'restartServer') {
    return {
      title: 'Restart Server',
      description: 'The service will restart and may be temporarily unavailable.',
      actionLabel: 'Restart',
      actionClass: 'bg-yellow-600 text-white hover:bg-yellow-500',
    }
  }
  return {
    title: 'Stop Server',
    description: 'The service will stop. You need to manually start it again.',
    actionLabel: 'Stop Server',
    actionClass: 'bg-red-600 text-white hover:bg-red-500',
  }
})

function formatTime(iso: string) {
  if (!iso) return '-'
  return new Date(iso).toLocaleTimeString()
}

function getLevelClass(level: string) {
  switch (level?.toLowerCase()) {
    case 'info':
      return 'text-blue-400'
    case 'warn':
      return 'text-yellow-400'
    case 'error':
      return 'text-red-400'
    case 'debug':
      return 'text-gray-500'
    case 'trace':
      return 'text-gray-600'
    default:
      return 'text-gray-400'
  }
}

async function refreshAll() {
  loadingAll.value = true
  notice.value = ''
  try {
    await Promise.all([loadInfo(), refreshLogs()])
    refreshKey.value++
    noticeType.value = 'success'
    notice.value = 'System information and logs refreshed.'
  } catch (e: any) {
    noticeType.value = 'error'
    notice.value = e?.message || 'Failed to refresh system data'
  } finally {
    loadingAll.value = false
  }
}

async function loadInfo() {
  try {
    const info = await api.getServerInfo()
    version.value = info.version || 'Unknown'
    schemes.value = await api.getSchemes()
  } catch (e: any) {
    noticeType.value = 'error'
    notice.value = e?.message || 'Failed to load system info'
  }
}

async function refreshLogs() {
  loadingLogs.value = true
  try {
    const data = await api.getLog()
    logs.value = data.reverse()
  } catch (e: any) {
    noticeType.value = 'error'
    notice.value = e?.message || 'Failed to load logs'
  } finally {
    loadingLogs.value = false
  }
}

function openConfirm(action: 'clearLogs' | 'restartServer' | 'stopServer') {
  confirmAction.value = action
  confirmOpen.value = true
}

async function clearLogs() {
  try {
    await api.clearLog()
    logs.value = []
    noticeType.value = 'success'
    notice.value = 'Logs cleared.'
  } catch (e: any) {
    noticeType.value = 'error'
    notice.value = e?.message || 'Failed to clear logs'
  }
}

async function restart() {
  try {
    await api.restartServer()
    noticeType.value = 'success'
    notice.value = 'Restart command sent. Please wait for the service to come back up.'
  } catch (e: any) {
    noticeType.value = 'error'
    notice.value = e?.message || `Failed to restart: ${String(e)}`
  }
}

async function exit() {
  try {
    api.exitServer().catch(() => {})
  } finally {
    isServerOffline.value = true
    startConnectionCheck()
  }
}

async function executeConfirmAction() {
  const action = confirmAction.value
  confirmOpen.value = false
  confirmAction.value = null
  if (action === 'clearLogs') {
    await clearLogs()
    return
  }
  if (action === 'restartServer') {
    await restart()
    return
  }
  if (action === 'stopServer') {
    await exit()
  }
}

function startConnectionCheck() {
  if (connectionCheckInterval) clearInterval(connectionCheckInterval)
  checkConnection()
  connectionCheckInterval = setInterval(checkConnection, 3000)
}

async function checkConnection() {
  reconnecting.value = true
  try {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 2000)
    await fetch('/api', { signal: controller.signal })
    clearTimeout(timeoutId)
    isServerOffline.value = false
    loadInfo()
    refreshLogs()
    if (connectionCheckInterval) {
      clearInterval(connectionCheckInterval)
      connectionCheckInterval = undefined
    }
  } catch (_) {
  } finally {
    reconnecting.value = false
  }
}

async function showStack() {
  try {
    stackTrace.value = 'Loading...'
    showStackModal.value = true
    stackTrace.value = await api.getStack()
  } catch (_) {
    stackTrace.value = 'Failed to load stack trace'
  }
}

function stopLogPolling() {
  if (pollInterval) {
    clearInterval(pollInterval)
    pollInterval = undefined
  }
}

function startLogPolling() {
  stopLogPolling()
  pollInterval = setInterval(refreshLogs, 5000)
}

watch(autoRefresh, (val) => {
  if (val) {
    refreshLogs()
    startLogPolling()
  } else {
    stopLogPolling()
  }
})

onMounted(() => {
  loadInfo()
  refreshLogs()
  if (autoRefresh.value) {
    startLogPolling()
  }
})

onUnmounted(() => {
  stopLogPolling()
  if (connectionCheckInterval) clearInterval(connectionCheckInterval)
})
</script>
