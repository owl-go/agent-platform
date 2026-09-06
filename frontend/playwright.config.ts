import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './e2e',
  timeout: 45000,
  expect: { timeout: 10000 },
  workers: 1,
  retries: 0,
  reporter: [['list'], ['html', { outputFolder: '../output/playwright/html', open: 'never' }], ['json', { outputFile: '../output/playwright/results.json' }]],
  outputDir: '../output/playwright/artifacts',
  use: { baseURL: process.env.E2E_BASE_URL, browserName: 'chromium', trace: 'off', screenshot: 'off', video: 'off' },
});
