import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv, type Plugin } from "vite";

function frontendVersionPlugin(version: string): Plugin {
  const payload = `${JSON.stringify({ version }, null, 2)}\n`;

  return {
    name: "frontend-version-plugin",
    configureServer(server) {
      server.middlewares.use("/version.json", (_request, response) => {
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.setHeader("Cache-Control", "no-store");
        response.end(payload);
      });
    },
    generateBundle() {
      this.emitFile({
        type: "asset",
        fileName: "version.json",
        source: payload,
      });
    },
  };
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ".", "");
  const frontendVersion = (env.VITE_APP_VERSION?.trim() || `dev-${new Date().toISOString()}`).replace(/"/g, "");

  return {
    plugins: [react(), frontendVersionPlugin(frontendVersion)],
    define: {
      __APP_VERSION__: JSON.stringify(frontendVersion),
    },
  };
});
