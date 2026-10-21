type ShutdownStep = { name: string; close: () => unknown }

export class ShutdownController {
  private pending: Promise<void> | undefined

  constructor(
    private steps: ShutdownStep[],
    private report: (name: string, error: unknown) => void,
    private timeout = 5000
  ) {}

  run(): Promise<void> {
    this.pending ||= this.close()
    return this.pending
  }

  private async close(): Promise<void> {
    for (const step of this.steps) {
      let timer: ReturnType<typeof setTimeout> | undefined
      try {
        await Promise.race([
          Promise.resolve().then(step.close),
          new Promise<never>((_, reject) => {
            timer = setTimeout(() => reject(new Error('Shutdown timed out')), this.timeout)
          })
        ])
      } catch (error) {
        this.report(step.name, error)
      } finally {
        clearTimeout(timer)
      }
    }
  }
}
