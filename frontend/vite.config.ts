import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
export default defineConfig({ plugins: [vue()], build: { chunkSizeWarningLimit: 1800 }, server: { proxy: { '/api': 'http://127.0.0.1:17839' } } })
