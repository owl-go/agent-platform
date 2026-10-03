import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import Components from "unplugin-vue-components/vite";
import { ElementPlusResolver } from "unplugin-vue-components/resolvers";
import { resolve } from "node:path";

export default defineConfig({
  plugins: [vue(), Components({ resolvers: [ElementPlusResolver()], dts: false }), {
    name: "assistant-embed-development-entry",
    configureServer(server) {
      server.middlewares.use((request, _response, next) => {
        if (request.url === "/assets/assistant-embed.js") request.url = "/src/assistant-embed.ts";
        next();
      });
    },
  }],
  build: {
    rolldownOptions: {
      input: { index: resolve(import.meta.dirname, "index.html"), "assistant-embed": resolve(import.meta.dirname, "src/assistant-embed.ts") },
      output: { entryFileNames: (chunk) => chunk.name === "assistant-embed" ? "assets/assistant-embed.js" : "assets/[name]-[hash].js" },
    },
  },
  server: {
    proxy: {
      "/api": {
        target: process.env.VITE_API_PROXY_TARGET ?? "http://127.0.0.1:8080",
        changeOrigin: false,
        rewrite: (path) => path.replace(/^\/api\/(healthz|readyz)(?=\?|$)/, "/$1"),
      },
      "/embed": {
        target: process.env.VITE_API_PROXY_TARGET ?? "http://127.0.0.1:8080",
        changeOrigin: false,
      },
    },
  },
});
