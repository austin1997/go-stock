import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { TDesignResolver } from '@tdesign-vue-next/auto-import-resolver'

const root = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      resolvers: [TDesignResolver({
        library: 'chat',
      })],
    }),
    Components({
      resolvers: [TDesignResolver({
        library: 'chat',
      })],
    }),
  ],
  define: {
    'import.meta.env.VITE_WEB': JSON.stringify('true'),
  },
  resolve: {
    alias: [
      {
        find: /(?:\.\.\/)*wailsjs\/go\/main\/App(?:\.js)?$/,
        replacement: path.resolve(root, 'src/platform/app-web.js'),
      },
      {
        find: /(?:\.\.\/)*wailsjs\/runtime(?:\/runtime)?(?:\.js)?$/,
        replacement: path.resolve(root, 'src/platform/runtime-web.js'),
      },
    ],
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
