export function dictationLevel(samples: Uint8Array): number {
  if (!samples.length) return 0
  let energy = 0
  for (const sample of samples) energy += ((sample - 128) / 128) ** 2
  return Math.min(1, Math.max(0, Math.sqrt(energy / samples.length) - 0.008) * 5)
}

export function smoothDictationLevel(current: number, target: number, seconds: number): number {
  const speed = target > current ? 24 : 7
  return current + (target - current) * (1 - Math.exp(-speed * Math.max(0, seconds)))
}
