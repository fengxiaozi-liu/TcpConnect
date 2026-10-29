import { defineConfig } from '@playwright/test'
export default defineConfig({
  testDir:'./e2e',workers:1,timeout:45000,
  use:{baseURL:'http://127.0.0.1:17839',channel:'msedge',headless:true,viewport:{width:1440,height:1000},screenshot:'only-on-failure',trace:'retain-on-failure'},
  outputDir:'../.local/playwright',
  webServer:{command:`go run . --web --port 17839 --data-dir .local/e2e-${Date.now()}`,cwd:'..',url:'http://127.0.0.1:17839',reuseExistingServer:false,timeout:120000}
})
