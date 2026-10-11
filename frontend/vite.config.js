import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const backendPort = process.env.PORT || '8080'
const backendTarget = `http://localhost:${backendPort}`

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3000,
    proxy: {
      '/repomanager.v1.RepoManagerService': {
        target: backendTarget,
        changeOrigin: true,
        secure: false,
      },
      '/api': {
        target: backendTarget,
        changeOrigin: true,
        secure: false,
      },
      '/metrics': {
        target: backendTarget,
        changeOrigin: true,
        secure: false,
      },
    },
  },
})
