import { defineConfig, devices } from '@playwright/test'

const baseURL = 'http://127.0.0.1:9246'

export default defineConfig({
  testDir: './e2e',
  outputDir: './test-results',
  fullyParallel: true,
  reporter: 'list',
  use: {
    baseURL,
    colorScheme: 'light',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'npm run dev -- --host 127.0.0.1 --port 9246',
    url: baseURL,
    reuseExistingServer: false,
    timeout: 60_000,
  },
})
