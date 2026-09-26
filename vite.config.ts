import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

// The workbench sandbox only runs inline or same-origin classic scripts, so the UI ships as one IIFE.
// ui/ is fully generated: public/index.html is copied next to app.js and app.css.
export default defineConfig({
  plugins: [vue()],
  define: { "process.env.NODE_ENV": JSON.stringify("production") },
  build: {
    outDir: "ui",
    emptyOutDir: true,
    target: "es2022",
    lib: { entry: "src/main.ts", formats: ["iife"], name: "NatsWorkbench", fileName: () => "app.js", cssFileName: "app" },
  },
});
