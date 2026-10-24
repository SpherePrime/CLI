import { dictationLevel, smoothDictationLevel } from './dictationSignal'

export function createDictationRenderer(
  canvas: HTMLCanvasElement,
  analyser: AnalyserNode | null,
  transcribing: boolean
) {
  const context = canvas.getContext('2d')
  if (!context) return () => {}
  const preference = matchMedia('(prefers-reduced-motion: reduce)')
  const samples = new Uint8Array(analyser?.fftSize || 256)
  const history = new Array<number>(180).fill(0)
  let frame = 0
  let last = performance.now()
  let sampleTime = 0
  let phase = 0
  let level = 0
  let width = 0
  let height = 0
  let color = ''

  const resize = () => {
    width = canvas.clientWidth
    height = canvas.clientHeight
    const ratio = Math.min(window.devicePixelRatio || 1, 2)
    canvas.width = Math.round(width * ratio)
    canvas.height = Math.round(height * ratio)
    context.setTransform(ratio, 0, 0, ratio, 0, 0)
  }
  const draw = (now: number) => {
    frame = 0
    const seconds = Math.min((now - last) / 1000, 0.05)
    last = now
    const reduced = preference.matches || document.documentElement.dataset.motion === 'reduced'
    if (!reduced) phase += seconds
    if (analyser && !transcribing) analyser.getByteTimeDomainData(samples)
    level = smoothDictationLevel(
      level,
      analyser && !transcribing ? dictationLevel(samples) : 0,
      seconds
    )
    sampleTime += seconds
    while (sampleTime >= 0.045) {
      history.shift()
      history.push(level)
      sampleTime -= 0.045
    }
    context.clearRect(0, 0, width, height)
    context.fillStyle = color
    const count = Math.max(1, Math.floor(width / 6))
    const offset = (width - (count - 1) * 6 - 3) / 2
    for (let index = 0; index < count; index++) {
      const position = index / Math.max(1, count - 1)
      const historyPosition = history.length - count + index - 1 + sampleTime / 0.045
      const lower = Math.max(0, Math.min(history.length - 1, Math.floor(historyPosition)))
      const upper = Math.min(history.length - 1, lower + 1)
      const mix = historyPosition - Math.floor(historyPosition)
      const recorded = history[lower] * (1 - mix) + history[upper] * mix
      const envelope = Math.sin(Math.PI * position) ** 0.6
      const processing =
        (0.22 + 0.28 * (0.5 + 0.5 * Math.sin(position * 10 - phase * 4))) * envelope
      const idle = 2 + Math.sin(position * 14 - phase * 2) * 0.7
      const amplitude = reduced
        ? 3
        : transcribing
          ? processing * (height - 6)
          : idle + recorded * (height - 8)
      const barHeight = Math.max(3, Math.min(height - 4, amplitude))
      context.globalAlpha = reduced
        ? 0.55
        : 0.22 + 0.78 * Math.sin(Math.PI * (0.08 + position * 0.84))
      context.beginPath()
      context.roundRect(offset + index * 6, (height - barHeight) / 2, 3, barHeight, 1.5)
      context.fill()
    }
    context.globalAlpha = 1
    if (!document.hidden && !reduced) frame = requestAnimationFrame(draw)
  }
  const restart = () => {
    cancelAnimationFrame(frame)
    last = performance.now()
    color = getComputedStyle(canvas).color
    if (!document.hidden) frame = requestAnimationFrame(draw)
  }
  const observer = new ResizeObserver(() => {
    resize()
    restart()
  })
  observer.observe(canvas)
  const themeObserver = new MutationObserver(restart)
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-motion', 'data-theme']
  })
  preference.addEventListener('change', restart)
  document.addEventListener('visibilitychange', restart)
  resize()
  restart()
  return () => {
    cancelAnimationFrame(frame)
    observer.disconnect()
    themeObserver.disconnect()
    preference.removeEventListener('change', restart)
    document.removeEventListener('visibilitychange', restart)
  }
}
