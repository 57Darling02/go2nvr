<template>
  <div class="flex h-screen w-screen bg-gray-900 text-white overflow-hidden">
    <aside
      class="w-16 md:w-44 shrink-0 flex flex-col border-r border-gray-800 bg-gray-950 transition-all duration-300"
    >
      <div
        class="h-16 flex items-center justify-center md:justify-start md:px-6 border-b border-gray-800"
      >
        <img src="/favicon.ico" alt="Go2NVR" class="w-8 h-8" />
        <span class="hidden md:block ml-3 font-bold text-lg tracking-wide">Go2NVR</span>
      </div>

      <TooltipProvider>
        <ScrollArea class="flex-1 py-3 px-2 md:px-3">
          <nav class="space-y-1">
            <Tooltip v-for="item in navItems" :key="item.to">
              <TooltipTrigger as-child>
                <Button as-child variant="ghost" :class="getNavButtonClass(item.to)">
                  <router-link :to="item.to">
                    <component :is="item.icon" class="w-5 h-5 shrink-0" />
                    <span class="hidden md:block ml-3">{{ item.label }}</span>
                  </router-link>
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right" class="md:hidden">
                {{ item.label }}
              </TooltipContent>
            </Tooltip>
          </nav>
        </ScrollArea>
      </TooltipProvider>

      <Separator class="bg-gray-800" />
      <div class="p-4">
        <div class="flex items-center justify-center md:justify-start text-gray-500 text-xs">
          <Info class="w-4 h-4 md:mr-2" />
          <span class="hidden md:block">{{ appVersion }}</span>
        </div>
      </div>
    </aside>

    <main class="flex-1 flex flex-col h-full overflow-hidden bg-gray-900 relative">
      <router-view v-slot="{ Component }">
        <transition name="fade" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue'
import { useRoute } from 'vue-router'
import { LayoutGrid, Settings, Info, Clock, Server, SlidersHorizontal } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

type NavItem = {
  to: string
  label: string
  icon: Component
}

const route = useRoute()
const appVersion = import.meta.env.VITE_APP_VERSION || 'dev'

const navItems: NavItem[] = [
  { to: '/', label: 'Dashboard', icon: LayoutGrid },
  { to: '/live-control', label: 'Live Control', icon: SlidersHorizontal },
  { to: '/recordings', label: 'Recordings', icon: Clock },
  { to: '/config', label: 'Configuration', icon: Settings },
  { to: '/system', label: 'System', icon: Server },
]

function isActivePath(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

function getNavButtonClass(path: string) {
  return cn(
    'w-full h-11 justify-center md:justify-start rounded-md px-2 md:px-3 text-gray-400 hover:text-white hover:bg-gray-800 border border-transparent',
    isActivePath(path) &&
      'text-blue-400 bg-gray-900 border-gray-800 md:rounded-r-none md:border-r-2 md:border-r-blue-500',
  )
}
</script>

<style>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
