import { defineConfig } from 'vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { workflowSource } from './dev/source-plugin.mjs';

export default defineConfig({
  cacheDir: 'node_modules/.vite-kit',
  plugins: [workflowSource(), sveltekit()],
  server: { host: '127.0.0.1', port: 8767, strictPort: true },
});
