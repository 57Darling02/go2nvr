<template>
  <div class="flex flex-col h-full bg-gray-900 text-white">
    <header
      class="flex flex-col gap-3 px-3 py-3 bg-gray-900 border-b border-gray-800 sm:flex-row sm:items-center sm:justify-between sm:px-6 sm:py-4"
    >
      <div class="flex items-center gap-2 sm:gap-3 min-w-0">
        <h1 class="text-base sm:text-xl font-semibold text-white truncate">Configuration</h1>
        <Button
          variant="ghost"
          size="icon"
          class="text-gray-400 hover:text-white"
          @click="load"
          title="Reload Configuration"
        >
          <RefreshCw class="w-5 h-5" :class="{ 'animate-spin': loading }" />
        </Button>
      </div>
      <div class="flex items-center gap-2 flex-wrap w-full sm:w-auto sm:justify-end">
        <Button
          as-child
          variant="outline"
          class="border-gray-700 bg-gray-800 text-gray-300 hover:bg-gray-700 hover:text-white w-full sm:w-auto"
        >
          <router-link to="/system">
            <Server class="w-4 h-4 mr-2" />
            System
          </router-link>
        </Button>
        <Button
          @click="save"
          class="bg-blue-600 text-white hover:bg-blue-500 flex-1 sm:flex-none"
          :disabled="saving"
        >
          <Save class="w-4 h-4 mr-2" />
          <span class="sm:hidden">{{ saving ? 'Saving...' : 'Save' }}</span>
          <span class="hidden sm:inline">{{ saving ? 'Saving...' : 'Save Config' }}</span>
        </Button>
      </div>
    </header>

    <div class="flex-1 min-h-0 p-3 sm:p-6">
      <div
        class="h-full min-h-0 flex flex-col rounded-lg border border-gray-800 bg-gray-950 overflow-hidden"
      >
        <div class="px-3 py-3 sm:px-4 border-b border-gray-800 bg-gray-900/40">
          <span v-if="configPath">Configuration path: {{ configPath }}</span>
          <p class="text-xs sm:text-sm text-gray-400">
            Edit configuration directly and save changes.
          </p>
        </div>

        <Alert
          v-if="notice"
          :variant="noticeType === 'error' ? 'destructive' : 'default'"
          :class="
            noticeType === 'error'
              ? 'm-3 sm:m-4 border-red-800 bg-red-900/40 text-red-200 [&_p]:text-red-200'
              : 'm-3 sm:m-4 border-green-800 bg-green-900/30 text-green-200 [&_p]:text-green-200'
          "
        >
          <AlertDescription>{{ notice }}</AlertDescription>
        </Alert>

        <div class="relative flex-1 min-h-0 p-3 pt-0 sm:p-4 sm:pt-0">
          <textarea
            v-model="config"
            :key="refreshKey"
            class="w-full h-full min-h-[320px] bg-gray-950 text-gray-200 font-mono text-xs sm:text-sm p-3 sm:p-4 border border-gray-800 rounded-md focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none resize-none"
            spellcheck="false"
          ></textarea>
          <div
            v-if="loading"
            class="absolute inset-0 flex items-center justify-center bg-gray-900/50 rounded-md"
          >
            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Save, Server, RefreshCw } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import * as api from '@/services/api'

const config = ref('')
const loading = ref(false)
const saving = ref(false)
const refreshKey = ref(0)
const notice = ref('')
const noticeType = ref<'success' | 'error'>('success')
const configPath = ref('')

async function load() {
  loading.value = true
  notice.value = ''
  try {
    const [yaml, serverInfo] = await Promise.all([api.getConfig(), api.getServerInfo()])
    config.value = yaml
    configPath.value = serverInfo.config_path?.trim() || ''
    refreshKey.value++
  } catch (e: any) {
    noticeType.value = 'error'
    notice.value = e?.message || `Failed to load config: ${String(e)}`
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  notice.value = ''
  try {
    await api.saveConfig(config.value)
    noticeType.value = 'success'
    notice.value = 'Configuration saved. Go2RTC may restart.'
  } catch (e: any) {
    noticeType.value = 'error'
    notice.value = e?.message || `Failed to save config: ${String(e)}`
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
