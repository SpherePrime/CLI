export function readOrbAudio(
  frequencies: Uint8Array,
  samples: Uint8Array,
  previousLevel: number,
  previousBands: number[]
) {
  let energy = 0
  for (const sample of samples) energy += ((sample - 128) / 128) ** 2
  const target = Math.min(1, Math.sqrt(energy / Math.max(1, samples.length)) * 4.5)
  const span = Math.max(1, Math.floor((frequencies.length * 0.6) / previousBands.length))
  const bands = previousBands.map((previous, band) => {
    let sum = 0
    let count = 0
    for (
      let index = band * span;
      index < Math.min((band + 1) * span, frequencies.length);
      index++
    ) {
      sum += frequencies[index]
      count++
    }
    return previous + (sum / Math.max(1, count) / 255 - previous) * 0.25
  })
  return {
    level: previousLevel + (target - previousLevel) * (target > previousLevel ? 0.35 : 0.12),
    bands
  }
}
