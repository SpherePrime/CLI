import { useEffect, useState } from 'react'

export type AudioDevice = {
  id: string
  label: string
}

// Device labels stay empty until the renderer holds microphone permission, so
// the hook briefly opens a stream to trigger the permission prompt before
// enumerating. Output devices only get labels once input permission exists —
// that is how Chromium gates device names, and Prime needs the mic anyway.
export function useAudioDevices() {
  const [inputs, setInputs] = useState<AudioDevice[]>([])
  const [outputs, setOutputs] = useState<AudioDevice[]>([])
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
      const name = (device: MediaDeviceInfo, index: number) => ({
        id: device.deviceId,
        label: device.label || `#${index + 1}`
      })
      setInputs(devices.filter((device) => device.kind === 'audioinput').map(name))
      setOutputs(devices.filter((device) => device.kind === 'audiooutput').map(name))
    }
    void enumerate()
    const handler = () => void enumerate()
    navigator.mediaDevices.addEventListener('devicechange', handler)
    return () => {
      cancelled = true
      navigator.mediaDevices.removeEventListener('devicechange', handler)
    }
  }, [])
  return { inputs, outputs }
}
