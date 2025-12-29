import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { readFileSync } from 'fs'
import { fileURLToPath } from 'url'
import { dirname, resolve } from 'path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

// Get version from environment or package.json
const getVersion = () => {
  if (process.env.APP_VERSION) {
    return process.env.APP_VERSION
  }
  try {
    const pkgPath = resolve(__dirname, 'package.json')
    const pkg = JSON.parse(readFileSync(pkgPath, 'utf-8'))
    return pkg.version || '0.0.0'
  } catch {
    return '0.0.0'
  }
}

const getCommitHash = () => {
  if (process.env.COMMIT_SHA) {
    return process.env.COMMIT_SHA.substring(0, 7)
  }
  return 'dev'
}

export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify(getVersion()),
    __COMMIT_HASH__: JSON.stringify(getCommitHash()),
    __BUILD_TIME__: JSON.stringify(new Date().toISOString())
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false
  }
})
