import { useEffect, useState } from 'react'

export type AudioDevice = {
  id: string
  label: string
}

// Chromium labels the follow-the-system-default entries "Default - <name>".
// Stripping that prefix turns them into the plain device name for the "Auto"
// option, which tracks the OS choice without pinning a device id.
const defaultPrefix = /^(default|communications)\s*-\s*/i

const plainLabel = (device: MediaDeviceInfo) => device.label.replace(defaultPrefix, '').trim()

// Device labels stay empty until the renderer holds microphone permission, so
// the hook briefly opens a stream to trigger the permission prompt before
// enumerating. Output devices only get labels once input permission exists —
// that is how Chromium gates device names, and Prime needs the mic anyway.
export function useAudioDevices() {
  const [inputs, setInputs] = useState<AudioDevice[]>([])
  const [outputs, setOutputs] = useState<AudioDevice[]>([])
  const [defaultInput, setDefaultInput] = useState('')
  const [defaultOutput, setDefaultOutput] = useState('')
  useEffect(() => {
    let cancelled = false
    const enumerate = async () => {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
        stream.getTracks().forEach((track) => track.stop())
      } catch {
        // Without permission the lists still load, with device ids as labels.
      }
      const devices = await navigator.mediaDevices.enumerateDevices()
      if (cancelled) return
      const named = (kind: 'audioinput' | 'audiooutput') =>
        devices
          .filter((device) => device.kind === kind)
          // "Default" and "Communications" entries mirror other list items
          // and carry no id of their own, so the real devices stay alone.
          .filter((device) => device.deviceId !== 'default' && device.deviceId !== 'communications')
          .map((device, index) => ({ id: device.deviceId, label: device.label || `#${index + 1}` }))
      setInputs(named('audioinput'))
      setOutputs(named('audiooutput'))
      setDefaultInput(
        plainLabel(
          devices.find((device) => device.kind === 'audioinput' && device.deviceId === 'default') ??
            ({ label: '' } as MediaDeviceInfo)
        )
      )
      setDefaultOutput(
        plainLabel(
          devices.find(
            (device) => device.kind === 'audiooutput' && device.deviceId === 'default'
          ) ?? ({ label: '' } as MediaDeviceInfo)
        )
      )
    }
    void enumerate()
    const handler = () => void enumerate()
    navigator.mediaDevices.addEventListener('devicechange', handler)
    return () => {
      cancelled = true
      navigator.mediaDevices.removeEventListener('devicechange', handler)
    }
  }, [])
  return { inputs, outputs, defaultInput, defaultOutput }
}
