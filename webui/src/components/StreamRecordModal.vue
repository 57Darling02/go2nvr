<template>
  <Dialog :open="props.open" @update:open="onOpenChange">
    <DialogContent
      class="h-[90vh] sm:max-w-lg p-0 bg-gray-900 border-gray-700 text-white flex flex-col overflow-hidden"
      :show-close-button="false"
    >
      <DialogHeader class="px-4 pt-4 sm:px-6 sm:pt-6 shrink-0">
        <div class="flex items-start justify-between gap-3">
          <DialogTitle class="text-base sm:text-lg text-white flex items-center flex-wrap">
            Record Rule
            <span class="mx-2 text-gray-600">|</span>
            <span class="text-blue-400">{{ stream }}</span>
            <span
              v-if="!loading"
              class="ml-3 text-xs px-2 py-0.5 rounded border"
              :class="
                hasRule
                  ? 'border-green-500/50 text-green-400 bg-green-500/10'
                  : 'border-gray-700 text-gray-500'
              "
            >
              {{ hasRule ? (form.triggerEnabled ? 'Trigger ON' : 'Recorder ON') : 'Disabled' }}
            </span>
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
          <Loader2 class="w-5 h-5 mr-2 animate-spin" />
          Loading rule...
        </div>

        <ScrollArea v-else class="h-full">
          <div class="pr-3">
            <form @submit.prevent="save" class="space-y-5">
              <Alert
                v-if="errorMessage"
                variant="destructive"
                class="border-red-800 bg-red-900/40 text-red-200 [&_p]:text-red-200"
              >
                <AlertDescription>{{ errorMessage }}</AlertDescription>
              </Alert>

              <div class="space-y-2">
                <Label class="text-gray-300">Rule</Label>
                <select
                  v-model="form.enabled"
                  class="w-full h-9 bg-gray-800 border border-gray-700 rounded-md px-3 text-white focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-colors"
                >
                  <option :value="false">Disabled</option>
                  <option :value="true">Enable Pre-buffer</option>
                </select>
              </div>

              <template v-if="form.enabled">
                <div class="space-y-2">
                  <Label class="text-gray-300">Pre-buffer (seconds)</Label>
                  <Input
                    type="number"
                    min="0"
                    v-model.number="form.prebuffer"
                    placeholder="10"
                    class="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500 focus-visible:ring-blue-500/40 focus-visible:border-blue-500"
                  />
                  <p class="text-xs text-gray-500">
                    Recorder keeps {{ form.prebuffer || 0 }} seconds in memory before manual start
                  </p>
                </div>

                <div class="space-y-2">
                  <Label class="text-gray-300">Trigger</Label>
                  <select
                    v-model="form.triggerEnabled"
                    class="w-full h-9 bg-gray-800 border border-gray-700 rounded-md px-3 text-white focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-colors"
                  >
                    <option :value="false">Disabled</option>
                    <option :value="true">Enabled</option>
                  </select>
                </div>

                <template v-if="form.triggerEnabled">
                  <div class="space-y-2">
                    <Label class="text-gray-300">Trigger Detector</Label>
                    <select
                      v-model.number="form.triggerId"
                      class="w-full h-9 bg-gray-800 border border-gray-700 rounded-md px-3 text-white focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-colors"
                    >
                      <option
                        v-for="trigger in triggerOptions"
                        :key="trigger.id"
                        :value="trigger.id"
                      >
                        {{ trigger.name }} ({{ trigger.key }})
                      </option>
                    </select>
                  </div>

                  <div class="space-y-2">
                    <Label class="text-gray-300">Interval (ms)</Label>
                    <Input
                      type="number"
                      min="100"
                      step="50"
                      v-model.number="form.triggerInterval"
                      class="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500 focus-visible:ring-blue-500/40 focus-visible:border-blue-500"
                    />
                  </div>

                  <div v-if="activeTriggerParams.length > 0" class="space-y-3">
                    <Label class="text-gray-300">Trigger Parameters</Label>
                    <div v-for="param in activeTriggerParams" :key="param.key" class="space-y-2">
                      <Label class="text-gray-300">{{ getParamLabel(param) }}</Label>
                      <select
                        v-if="isBooleanParam(param)"
                        :value="String(getBooleanParamValue(param.key))"
                        @change="onBooleanParamInput(param.key, $event)"
                        class="w-full h-9 bg-gray-800 border border-gray-700 rounded-md px-3 text-white focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-colors"
                      >
                        <option value="false">False</option>
                        <option value="true">True</option>
                      </select>
                      <Input
                        v-else-if="isNumberParam(param)"
                        type="number"
                        :min="param.min"
                        :max="param.max"
                        :model-value="getNumberParamValue(param.key)"
                        :placeholder="getParamPlaceholder(param)"
                        @update:model-value="setNumberParamValue(param, String($event))"
                        class="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500 focus-visible:ring-blue-500/40 focus-visible:border-blue-500"
                      />
                      <Input
                        v-else
                        type="text"
                        :model-value="getStringParamValue(param.key)"
                        :placeholder="getParamPlaceholder(param)"
                        @update:model-value="setStringParamValue(param.key, String($event))"
                        class="bg-gray-800 border-gray-700 text-white placeholder:text-gray-500 focus-visible:ring-blue-500/40 focus-visible:border-blue-500"
                      />
                      <p v-if="param.tip" class="text-xs text-gray-500">{{ param.tip }}</p>
                    </div>
                  </div>
                </template>
              </template>

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
                  {{ saving ? 'Saving...' : 'Save Configuration' }}
                </Button>
              </DialogFooter>
            </form>
          </div>
        </ScrollArea>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
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
import { ScrollArea } from '@/components/ui/scroll-area'
import * as api from '@/services/api'

const props = withDefaults(defineProps<{ stream: string; open?: boolean }>(), {
  open: false,
})
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
  (e: 'update:open', value: boolean): void
}>()

const loading = ref(true)
const saving = ref(false)
const hasRule = ref(false)
const triggerOptions = ref<api.TriggerMeta[]>([])
const errorMessage = ref('')

const form = reactive<{
  enabled: boolean
  prebuffer: number
  triggerEnabled: boolean
  triggerId: number
  triggerInterval: number
  triggerParams: Record<string, any>
}>({
  enabled: false,
  prebuffer: 10,
  triggerEnabled: true,
  triggerId: 1,
  triggerInterval: 250,
  triggerParams: {},
})

function resetForm() {
  form.enabled = false
  form.prebuffer = 10
  form.triggerEnabled = true
  form.triggerId = 1
  form.triggerInterval = 250
  form.triggerParams = {}
}

function getNumber(input: any, fallback: number) {
  const n = Number(input)
  return Number.isFinite(n) ? n : fallback
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

function getTriggerMetaById(triggerId: number) {
  return triggerOptions.value.find((t) => t.id === triggerId)
}

function isNumberParam(param: api.TriggerParamMeta) {
  const type = String(param.type || 'number').toLowerCase()
  return type === 'number' || type === 'int' || type === 'integer' || type === 'float'
}

function isBooleanParam(param: api.TriggerParamMeta) {
  return String(param.type || '').toLowerCase() === 'boolean'
}

function getParamLabel(param: api.TriggerParamMeta) {
  return param.name?.trim() || param.key
}

function coerceParamValue(param: api.TriggerParamMeta, value: any) {
  if (isBooleanParam(param))
    return value === true || value === 'true' || value === 1 || value === '1'
  if (isNumberParam(param)) {
    const numeric = Number(value)
    if (!Number.isFinite(numeric)) {
      const fallback = Number(param.default)
      return Number.isFinite(fallback) ? fallback : 0
    }
    return numeric
  }
  return value == null ? '' : String(value)
}

function getDefaultParamValue(param: api.TriggerParamMeta) {
  if (param.default !== undefined) return coerceParamValue(param, param.default)
  if (isBooleanParam(param)) return false
  if (isNumberParam(param)) return 0
  return ''
}

function isEmptyParamValue(value: any) {
  return value === undefined || value === null || value === ''
}

function getParamPlaceholder(param: api.TriggerParamMeta) {
  const defaultValue = getDefaultParamValue(param)
  return defaultValue === '' ? '' : String(defaultValue)
}

function hydrateTriggerParams(triggerId: number, source: Record<string, any> = {}) {
  const trigger = getTriggerMetaById(triggerId)
  const schema = Array.isArray(trigger?.params) ? trigger!.params! : []
  const next: Record<string, any> = {}
  for (const param of schema) {
    const raw = source[param.key]
    next[param.key] = raw === undefined ? '' : coerceParamValue(param, raw)
  }
  form.triggerParams = next
}

const activeTriggerParams = computed(() => {
  const trigger = getTriggerMetaById(form.triggerId)
  return Array.isArray(trigger?.params) ? trigger!.params! : []
})

function getNumberParamValue(key: string) {
  const raw = form.triggerParams[key]
  if (isEmptyParamValue(raw)) return ''
  const value = Number(raw)
  return Number.isFinite(value) ? value : ''
}

function setNumberParamValue(param: api.TriggerParamMeta, rawValue: string) {
  if (rawValue.trim() === '') {
    form.triggerParams[param.key] = ''
    return
  }
  const numeric = Number(rawValue)
  if (!Number.isFinite(numeric)) {
    form.triggerParams[param.key] = ''
    return
  }
  let value = numeric
  if (Number.isFinite(Number(param.min))) value = Math.max(Number(param.min), value)
  if (Number.isFinite(Number(param.max))) value = Math.min(Number(param.max), value)
  form.triggerParams[param.key] = value
}

function getBooleanParamValue(key: string) {
  return form.triggerParams[key] === true
}

function setBooleanParamValue(key: string, rawValue: string) {
  form.triggerParams[key] = rawValue === 'true'
}

function getStringParamValue(key: string) {
  const value = form.triggerParams[key]
  return value == null ? '' : String(value)
}

function setStringParamValue(key: string, rawValue: string) {
  form.triggerParams[key] = rawValue
}

function onBooleanParamInput(key: string, event: Event) {
  const target = event.target as HTMLSelectElement | null
  setBooleanParamValue(key, target?.value ?? 'false')
}

async function loadRule() {
  loading.value = true
  hasRule.value = false
  errorMessage.value = ''
  resetForm()
  try {
    const triggerList = await api.getRecordingTriggers()
    triggerOptions.value = Array.isArray(triggerList) ? triggerList : []
    if (triggerOptions.value.length === 0) {
      triggerOptions.value = [
        {
          id: 1,
          key: 'simple_diff',
          name: 'Simple Diff',
          params: [
            {
              key: 'threshold',
              type: 'number',
              default: 14,
              min: 1,
              max: 255,
              tip: 'Average grayscale diff threshold treated as motion.',
            },
            {
              key: 'post_sec',
              type: 'number',
              default: 10,
              min: 1,
              tip: 'Keep recording for N seconds after last detected motion.',
            },
            {
              key: 'min_hits',
              type: 'number',
              default: 1,
              min: 1,
              tip: 'Consecutive motion hits required before entering active state.',
            },
          ],
        },
      ]
    }
    if (!triggerOptions.value.some((t) => t.id === form.triggerId)) {
      form.triggerId = triggerOptions.value[0]?.id || 1
    }

    const rules = await api.getRecordingRules(props.stream)
    let currentRule: any
    if (Array.isArray(rules)) {
      currentRule = rules.find((r: any) => (r.src || r.Src) === props.stream)
    } else if (rules && typeof rules === 'object') {
      const r = rules as any
      const ruleSrc = r.src || r.Src
      if (ruleSrc === props.stream) {
        currentRule = r
      } else if (!ruleSrc && (r.prebuffer ?? r.Prebuffer) !== undefined) {
        currentRule = { ...r, src: props.stream }
      }
    }

    if (currentRule) {
      hasRule.value = true
      form.enabled = true
      const prebuffer = currentRule.prebuffer ?? currentRule.Prebuffer
      if (prebuffer !== undefined) form.prebuffer = Number(prebuffer) || 0
      const triggerId =
        currentRule.trigger_id ??
        currentRule.TriggerId ??
        currentRule.trigger ??
        currentRule.Trigger
      const triggerInterval = currentRule.trigger_interval ?? currentRule.TriggerInterval
      const triggerParams = currentRule.trigger_params ?? currentRule.TriggerParams
      form.triggerEnabled = getNumber(triggerId, 0) > 0
      const normalizedTriggerId = Math.max(0, getNumber(triggerId, form.triggerId))
      if (normalizedTriggerId > 0) form.triggerId = normalizedTriggerId
      if (triggerInterval !== undefined)
        form.triggerInterval = Math.max(100, getNumber(triggerInterval, 250))
      const legacyParams: Record<string, any> = {}
      const triggerThreshold = currentRule.trigger_threshold ?? currentRule.TriggerThreshold
      const triggerPost = currentRule.trigger_post ?? currentRule.TriggerPost
      if (triggerThreshold !== undefined) legacyParams.threshold = triggerThreshold
      if (triggerPost !== undefined) legacyParams.post_sec = triggerPost
      hydrateTriggerParams(form.triggerId, {
        ...legacyParams,
        ...(triggerParams && typeof triggerParams === 'object' ? triggerParams : {}),
      })
    } else {
      hydrateTriggerParams(form.triggerId)
    }
  } catch (e) {
    console.log('No existing rules found or error loading rules', e)
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.open, props.stream] as const,
  ([open, stream]) => {
    if (!open || !stream) return
    loadRule()
  },
  { immediate: true },
)

watch(
  () => form.triggerId,
  () => {
    hydrateTriggerParams(form.triggerId, form.triggerParams)
  },
)

async function save() {
  saving.value = true
  errorMessage.value = ''
  try {
    if (!form.enabled) {
      await api.deleteRecordingRule(props.stream)
      hasRule.value = false
    } else {
      const normalizedPrebuffer = Number.isFinite(form.prebuffer) ? Math.max(0, form.prebuffer) : 0
      const normalizedTriggerInterval = Number.isFinite(form.triggerInterval)
        ? Math.max(100, form.triggerInterval)
        : 250
      const normalizedTriggerId = Number.isFinite(form.triggerId)
        ? Math.max(0, Math.floor(form.triggerId))
        : 1
      const rule: api.RecordingRule = {
        src: props.stream,
        prebuffer: normalizedPrebuffer,
      }
      if (form.triggerEnabled) {
        rule.trigger_id = normalizedTriggerId > 0 ? normalizedTriggerId : 1
        rule.trigger_interval = normalizedTriggerInterval
        const trigger = getTriggerMetaById(rule.trigger_id)
        const schema = Array.isArray(trigger?.params) ? trigger!.params! : []
        const triggerParams: Record<string, any> = {}
        for (const param of schema) {
          const value = form.triggerParams[param.key]
          triggerParams[param.key] = isEmptyParamValue(value)
            ? getDefaultParamValue(param)
            : coerceParamValue(param, value)
        }
        rule.trigger_params = triggerParams
        if (triggerParams.threshold !== undefined) {
          const threshold = Number(triggerParams.threshold)
          if (Number.isFinite(threshold)) rule.trigger_threshold = Math.max(0, threshold)
        }
        if (triggerParams.post_sec !== undefined) {
          const postSec = Number(triggerParams.post_sec)
          if (Number.isFinite(postSec)) rule.trigger_post = Math.max(0, postSec)
        }
      } else {
        rule.trigger_id = 0
        rule.trigger_params = {}
      }
      await api.addRecordingRule(rule)
      hasRule.value = true
    }
    emit('saved')
    closeModal()
  } catch (e: any) {
    errorMessage.value = e?.message || 'Failed to save configuration'
  } finally {
    saving.value = false
  }
}
</script>
