import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  base: './',
  build: {
    outDir: fileURLToPath(new URL('../internal/nvrui/dist', import.meta.url)),
    emptyOutDir: true,
  },
  plugins: [
    vue({
      template: {
        compilerOptions: {
          isCustomElement: (tag) => tag === 'video-rtc'
        }
      }
    }),
    vueDevTools(),
    tailwindcss(),
  ],
  server: {
    proxy: {
      '/api/ws': {
        target: 'ws://127.0.0.1:1984',
        ws: true,
        changeOrigin: true,
        // Use configure to rewrite Origin header for WebSocket
        configure: (proxy) => {
          proxy.on('proxyReqWs', (proxyReq) => {
            proxyReq.setHeader('Origin', 'http://127.0.0.1:1984')
          })
        }
      },
      '/api': {
        target: 'http://127.0.0.1:1984',
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on('proxyReq', (proxyReq) => {
            proxyReq.setHeader('Origin', 'http://127.0.0.1:1984')
          })
        }
      }
    }
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
})
