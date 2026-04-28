<template>
  <Dialog :open="props.open" @update:open="onOpenChange">
    <DialogContent
      class="max-h-[92vh] sm:max-w-md p-0 bg-gray-900 border-gray-700 text-white overflow-hidden"
      :show-close-button="false"
    >
      <DialogHeader class="px-4 pt-4 sm:px-6 sm:pt-6">
        <div class="flex items-start justify-between gap-3">
          <DialogTitle class="text-base sm:text-lg text-white leading-6"
            >Global Recording Settings</DialogTitle
          >
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

      <div class="px-4 pb-4 sm:px-6 sm:pb-6 overflow-y-auto max-h-[calc(92vh-5.5rem)]">
        <div v-if="loading" class="flex items-center justify-center py-8 text-gray-400">
          <Loader2 class="w-5 h-5 mr-2 animate-spin" />
          Loading settings...
        </div>

        <form v-else @submit.prevent="save" class="space-y-5">
          <Alert
            v-if="errorMessage"
            variant="destructive"
            class="border-red-800 bg-red-900/40 text-red-200 [&_p]:text-red-200"
          >
            <AlertDescription>{{ errorMessage }}</AlertDescription>
          </Alert>

          <div class="space-y-2">
            <Label class="text-gray-300">Recording Directory</Label>
            <Input
              type="text"
              v-model="form.dir"
              placeholder="./records"
              class="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500 focus-visible:ring-blue-500/40 focus-visible:border-blue-500"
            />
            <p class="text-xs text-gray-500">Absolute path or relative to go2rtc binary</p>
          </div>

          <div class="space-y-2">
            <Label class="text-gray-300">Retention (Days)</Label>
            <Input
              type="number"
              v-model.number="form.retention"
              placeholder="7"
              min="0"
              class="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500 focus-visible:ring-blue-500/40 focus-visible:border-blue-500"
            />
            <p class="text-xs text-gray-500">
              Files older than this will be automatically deleted. Set to 0 to disable.
            </p>
          </div>

          <DialogFooter class="pt-4 mt-2 border-t border-gray-800 gap-2 sm:gap-3">
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
              :disabled="saving"
              class="bg-blue-600 text-white hover:bg-blue-500"
            >
              <Loader2 v-if="saving" class="w-4 h-4 mr-2 animate-spin" />
              <Save v-else class="w-4 h-4 mr-2" />
              {{ saving ? 'Saving...' : 'Save Settings' }}
            </Button>
          </DialogFooter>
        </form>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { Loader2, Save, X } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
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
  (e: 'saved'): void
  (e: 'update:open', value: boolean): void
}>()

const loading = ref(true)
const saving = ref(false)
const errorMessage = ref('')

// Form state
const form = reactive<{
  dir: string
  retention: number
}>({
  dir: './records',
  retention: 7,
})

async function loadConfig() {
  loading.value = true
  errorMessage.value = ''
  form.dir = './records'
  form.retention = 7
  try {
    const config = await api.getRecordingConfig()
    const c = config as any
    form.dir = c.dir || c.Dir || './records'
    form.retention = c.retention ?? c.Retention ?? 7
  } catch (e) {
    console.error('Failed to load recording config:', e)
  } finally {
    loading.value = false
  }
}

function onOpenChange(value: boolean) {
  emit('update:open', value)
  if (!value) {
    emit('close')
  }
}

function closeModal() {
  onOpenChange(false)
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    loadConfig()
  },
  { immediate: true },
)

async function save() {
  saving.value = true
  errorMessage.value = ''
  try {
    await api.updateRecordingConfig({
      dir: form.dir || './records',
      retention: form.retention ?? 7,
    })
    emit('saved')
    closeModal()
  } catch (e: any) {
    errorMessage.value = e?.message || 'Failed to save configuration'
  } finally {
    saving.value = false
  }
}
</script>
