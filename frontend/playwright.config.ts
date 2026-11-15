import { defineConfig, devices } from '@playwright/test'

const reuseExistingServer = !process.env.CI

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: false,
  workers: 1,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:5273',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      command: 'go -C ../backend run ./cmd/server -addr 127.0.0.1:8080',
      url: 'http://127.0.0.1:8080/health',
      reuseExistingServer,
      timeout: 120_000,
    },
    {
      command: 'npm run dev -- --port 5273 --strictPort',
      url: 'http://localhost:5273',
      reuseExistingServer,
      timeout: 120_000,
    },
  ],
})
