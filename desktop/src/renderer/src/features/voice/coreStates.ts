import type { ConversationState } from './useConversation'

export type CoreLook = {
  deep: [number, number, number]
  mid: [number, number, number]
  hot: [number, number, number]
  swirl: number
  flare: number
  ring: number
  ripple: number
  speed: number
}

const hex = (value: string): [number, number, number] => {
  const number = Number.parseInt(value.slice(1), 16)
  return [((number >> 16) & 255) / 255, ((number >> 8) & 255) / 255, (number & 255) / 255]
}

const ember = { deep: hex('#3a0a0c'), mid: hex('#ff5a2c'), hot: hex('#ffa65a') }
const thought = { deep: hex('#140b33'), mid: hex('#7d5cff'), hot: hex('#ff9fd4') }
const voice = { deep: hex('#300a18'), mid: hex('#ff4d6d'), hot: hex('#ff9c74') }

const looks: Record<ConversationState, CoreLook> = {
  off: { ...ember, swirl: 0.2, flare: 0.1, ring: 0, ripple: 0, speed: 0.4 },
  preparing: { ...thought, swirl: 1.4, flare: 0.25, ring: 0.9, ripple: 0, speed: 1 },
  listening: { ...ember, swirl: 0.35, flare: 0.25, ring: 0, ripple: 0, speed: 0.7 },
  hearing: { ...ember, swirl: 0.55, flare: 0.65, ring: 0, ripple: 0, speed: 1.05 },
  transcribing: { ...thought, swirl: 1.2, flare: 0.3, ring: 0.7, ripple: 0, speed: 1.1 },
  waiting: { ...thought, swirl: 1.6, flare: 0.35, ring: 1, ripple: 0, speed: 1.2 },
  speaking: { ...voice, swirl: 0.6, flare: 0.5, ring: 0, ripple: 1, speed: 1 }
}

export function coreLook(state: ConversationState): CoreLook {
  return looks[state]
}

const mixArray = (from: number[], to: number[], amount: number) =>
  from.map((value, index) => value + (to[index] - value) * amount) as [number, number, number]

export function blendLook(from: CoreLook, to: CoreLook, amount: number): CoreLook {
  const mix = (a: number, b: number) => a + (b - a) * amount
  return {
    deep: mixArray(from.deep, to.deep, amount),
    mid: mixArray(from.mid, to.mid, amount),
    hot: mixArray(from.hot, to.hot, amount),
    swirl: mix(from.swirl, to.swirl),
    flare: mix(from.flare, to.flare),
    ring: mix(from.ring, to.ring),
    ripple: mix(from.ripple, to.ripple),
    speed: mix(from.speed, to.speed)
  }
}
