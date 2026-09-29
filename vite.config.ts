import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
// Only the Vite server reads this value; browser requests remain relative /api.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const target = env.APP_ORIGIN || "http://127.0.0.1:3000";
  return {
    plugins: [react()],
    server: {
      port: 5173,
      strictPort: true,
      proxy: {
        "/api": { target, changeOrigin: false },
        "/health": { target, changeOrigin: false },
      },
    },
  };
});
