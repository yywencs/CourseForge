import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import { defineConfig, loadEnv } from 'vite'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const backendTarget = env.VITE_DEV_PROXY_TARGET || 'http://127.0.0.1:8080'
  const adminBackendTarget =
    env.VITE_DEV_ADMIN_PROXY_TARGET || 'http://127.0.0.1:8081'

  return {
    plugins: [
      vue({
        template: {
          compilerOptions: {
            comments: true,
          },
        },
      }),
      tailwindcss(),
      Components({
        dts: 'src/components.d.ts',
        resolvers: [ElementPlusResolver()],
      }),
    ],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      host: '0.0.0.0',
      port: 5173,
      proxy: {
        '/api': {
          target: backendTarget,
          changeOrigin: true,
        },
        '/health': {
          target: backendTarget,
          changeOrigin: true,
        },
        '/ready': {
          target: backendTarget,
          changeOrigin: true,
        },
        '/admin-api': {
          target: adminBackendTarget,
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/admin-api/, ''),
        },
      },
    },
  }
})
