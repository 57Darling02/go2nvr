import './assets/main.css'

import { createApp } from 'vue'
import { VideoRTC } from '@/lib/video-rtc.js'

import App from './App.vue'
import router from './router'

if (!customElements.get('video-rtc')) {
  customElements.define('video-rtc', VideoRTC)
}

const app = createApp(App)

app.use(router)

app.mount('#app')
