import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:3000",
    trace: "retain-on-failure",
  },
  webServer: [
    {
      command: "node e2e/mock-api.mjs",
      url: "http://127.0.0.1:18082/v1/status",
      reuseExistingServer: false,
      timeout: 30_000,
    },
    {
      command: "pnpm start",
      url: "http://127.0.0.1:3000",
      reuseExistingServer: false,
      timeout: 120_000,
      env: {
        HOSTNAME: "127.0.0.1",
        PORT: "3000",
        API_BASE_URL: "http://127.0.0.1:18082",
      },
    },
  ],
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
