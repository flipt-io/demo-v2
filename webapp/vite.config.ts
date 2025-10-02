import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    proxy: {
      "/internal/v1": "http://localhost:8080/",
    },
    origin: "http://localhost:5173",
  },
});
