import react from "@vitejs/plugin-react";
import { loadEnv } from "vite";
import { defineConfig } from "vitest/config";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ".", "");
  const frontendVersion = env.VITE_APP_VERSION?.trim() || "test-version";

  return {
    plugins: [react()],
    define: {
      __APP_VERSION__: JSON.stringify(frontendVersion),
    },
    test: {
      environment: "jsdom",
      setupFiles: "./src/test/setup.ts",
    },
  };
});
