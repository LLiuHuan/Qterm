import { wailsTypeScriptBindings } from './build/wails-bindings.ts'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import wails from '@wailsio/runtime/plugins/vite'

export default defineConfig({
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  resolve: {
    alias: {
      '@': new URL('./src', import.meta.url).pathname,
    },
  },
  plugins: [wailsTypeScriptBindings(), vue(), tailwindcss(), wails('./bindings')],

  worker: {
    format: 'es',
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (/[/\\]node_modules[/\\]@wailsio[/\\]runtime[/\\]/.test(id)) return 'wails-runtime'
          if (/[/\\]bindings[/\\]/.test(id)) return 'wails-runtime'
          return undefined
        },
      },
    },
  },
})
