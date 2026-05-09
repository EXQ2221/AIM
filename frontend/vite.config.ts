import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const gatewayTarget = process.env.VITE_GATEWAY_TARGET || "http://127.0.0.1:8080";
const wsTarget = gatewayTarget.replace(/^http/, "ws");

export default defineConfig({
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: {
      "/api": {
        target: gatewayTarget,
        changeOrigin: true
      },
      "/healthz": {
        target: gatewayTarget,
        changeOrigin: true
      },
      "/uploads": {
        target: gatewayTarget,
        changeOrigin: true
      },
      "/ws": {
        target: wsTarget,
        changeOrigin: true,
        ws: true
      }
    }
  }
});
