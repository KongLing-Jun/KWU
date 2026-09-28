import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    include: ["src/vitest/**/*.vitest.test.{ts,tsx}"],
    setupFiles: ["src/vitest/setup.ts"],
    clearMocks: true,
    restoreMocks: true,
    coverage: {
      provider: "v8",
      reportsDirectory: "../../output/vitest-coverage",
      reporter: ["text-summary", "json-summary", "html"],
      include: [
        "src/components/HistoryLoadErrorBanner.tsx",
        "src/components/SendAvailabilityBanner.tsx",
        "src/features/account/store/accountStore.ts",
        "src/lib/sendAvailability.ts",
      ],
    },
  },
});
