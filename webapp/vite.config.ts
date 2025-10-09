import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    proxy: {
      "/internal/v1/metrics": "http://localhost:4000/",
      "/internal/v1": "http://localhost:8080/",
      "/api/": "http://localhost:8000/",
    },
    origin: "http://localhost:5173",
  },
});
