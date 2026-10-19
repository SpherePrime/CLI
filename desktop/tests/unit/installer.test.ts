import { expect, it, vi } from 'vitest'
import { InstallerController } from '../../installer/src/main/controller'

it('installs the bundled payload once and verifies the CLI before reporting completion', async () => {
  const verifyPayload = vi.fn(async () => {})
  let finish: () => void = () => {}
  const launch = vi.fn(
    () =>
      new Promise<void>((resolve) => {
        finish = resolve
      })
  )
  const verifyInstall = vi.fn(async () => {})
  const phases: string[] = []
  const prepareVoice = vi.fn(async (progress: (percent: number) => void) => progress(100))
  const controller = new InstallerController({ verifyPayload, launch, verifyInstall, prepareVoice }, (status) =>
    phases.push(status.phase)
  )
  const install = controller.install('C:\\Apps\\Prime')
  await vi.waitFor(() => expect(launch).toHaveBeenCalledTimes(1))
  await expect(controller.install('C:\\Apps\\Other')).rejects.toThrow()
  finish()
  await install
  expect(verifyInstall).toHaveBeenCalledWith('C:\\Apps\\Prime')
  expect(phases).toEqual(['preparing', 'installing', 'verifying', 'voice', 'voice', 'complete'])
})
it('does not execute a corrupt payload or claim that an incomplete installation is ready', async () => {
  const launch = vi.fn()
  const controller = new InstallerController(
    {
      verifyPayload: async () => {
        throw new Error('Checksum mismatch')
      },
      launch,
      verifyInstall: vi.fn(),
      prepareVoice: vi.fn()
    },
    () => {}
  )
  expect(await controller.install('C:\\Apps\\Prime')).toMatchObject({ phase: 'error' })
  expect(launch).not.toHaveBeenCalled()
})
