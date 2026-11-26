import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import path from 'path'

// /api goes to nosd, in `vite` (dev) and `vite preview` (the built UI) alike.
// NOS_API_TARGET: scripts/live-ui points it at its container.
const apiProxy = {
	'/api': {
		target: process.env.NOS_API_TARGET || 'http://127.0.0.1:9000',
		changeOrigin: true,
	},
}

export default defineConfig({
	plugins: [react()],
	test: {
		environment: 'jsdom',
		globals: true,
		setupFiles: './vitest.setup.ts',
	},
	resolve: { alias: { '@': path.resolve(__dirname, 'src') } },
	server: { proxy: apiProxy },
	preview: { proxy: apiProxy },
})
