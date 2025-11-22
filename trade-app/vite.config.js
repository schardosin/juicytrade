import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import path from "path";

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3001,
    proxy: {
      "/api": {
        target: process.env.JUICYTRADE_API_BASE_URL || "http://localhost:8008",
        changeOrigin: true,
        // No rewrite - backend expects /api prefix
      },
      "/auth": {
        target: process.env.JUICYTRADE_API_BASE_URL || "http://localhost:8008",
        changeOrigin: true,
        // Forward /auth requests directly to backend (no rewrite needed)
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  envPrefix: ['VITE_', 'JUICYTRADE_'],
});
