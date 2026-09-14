import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite-plus';
import skgo from '@skgo/sveltekit-adapter';
import tailwindcss from '@tailwindcss/vite';

// The app's origin is fixed here, at build time, and the Go server checks it on
// every non-GET remote-function call. The two halves must agree, so both read
// the same ORIGIN: `just build` exports it from Justfile and bakes the same
// value into the binary. Change it in Justfile, nowhere else, and rebuild both
// halves together — a frontend served from one origin and a binary trusting
// another answers every command with 403.
export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			adapter: skgo(),
			paths: { origin: process.env.ORIGIN ?? 'http://127.0.0.1:8080' },
			experimental: { remoteFunctions: true },
			compilerOptions: { experimental: { async: true } }
		})
	]
});
