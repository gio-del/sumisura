import path from 'node:path'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.BACKEND_URL ?? 'http://localhost:8080',
      },
    },
  },
  // Vitest lives in this config (rather than its own) so tests inherit the
  // '@' alias and plugin setup above instead of restating them (ADR-0019).
  // globals: false keeps describe/it/expect explicit imports, so test files
  // type-check under the existing `tsc -b` with no `types` array surgery.
  test: {
    globals: false,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    // Stylesheets stay empty in tests, except theme.test.ts, which reads the
    // real tokens through ?raw.
    css: { include: [/index\.css\?raw/] },
    restoreMocks: true,
  },
})
