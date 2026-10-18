import { configDefaults, defineConfig } from 'vitest/config'
export default defineConfig({
  test: {
    environment: 'node',
    testTimeout: 20000,
    exclude: [...configDefaults.exclude, '.superpowers/**', 'release/**', 'out/**']
  }
})
