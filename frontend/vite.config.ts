import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

// The app runs inside WebKitGTK (webkit2gtk3 on RHEL 8/9), so target a
// Safari-level engine rather than the latest browsers.
export default defineConfig({
  plugins: [tailwindcss(), svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: ['es2020', 'safari16'],
  },
})
