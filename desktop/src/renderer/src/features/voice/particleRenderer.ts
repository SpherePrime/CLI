import { particleFragment, particleVertex } from './particleShader'
import type { CoreLook } from './coreStates'

export type ParticleUniforms = {
  time: number
  energy: number
  bass: number
  treble: number
  dim: number
  light: number
  bands: number[]
  look: CoreLook
}

const count = 8000
const uniformNames = [
  'time',
  'energy',
  'bass',
  'treble',
  'swirl',
  'flare',
  'ring',
  'ripple',
  'dim',
  'light',
  'pixel',
  'deep',
  'mid',
  'hot',
  'bandsA',
  'bandsB',
  'bandsC'
] as const

function particles() {
  const directions = new Float32Array(count * 3)
  const seeds = new Float32Array(count)
  const lanes = new Float32Array(count)
  const golden = Math.PI * (3 - Math.sqrt(5))
  for (let index = 0; index < count; index++) {
    const y = 1 - ((index + 0.5) / count) * 2
    const radius = Math.sqrt(1 - y * y)
    const angle = index * golden
    directions.set([Math.cos(angle) * radius, y, Math.sin(angle) * radius], index * 3)
    seeds[index] = Math.random()
    lanes[index] = index % 3
  }
  return { directions, seeds, lanes }
}

export function createParticleRenderer(canvas: HTMLCanvasElement) {
  const gl = canvas.getContext('webgl', { alpha: true, premultipliedAlpha: true, antialias: true })
  if (!gl) return null
  const shaders: WebGLShader[] = []
  const buffers: WebGLBuffer[] = []
  const program = gl.createProgram()
  const dispose = () => {
    shaders.forEach((shader) => gl.deleteShader(shader))
    buffers.forEach((buffer) => gl.deleteBuffer(buffer))
    gl.deleteProgram(program)
  }
  const compile = (type: number, source: string) => {
    const shader = gl.createShader(type)
    if (!shader) throw new Error('Voice shader allocation failed')
    shaders.push(shader)
    gl.shaderSource(shader, source)
    gl.compileShader(shader)
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS))
      throw new Error(gl.getShaderInfoLog(shader) || 'Voice shader compilation failed')
    return shader
  }
  const attribute = (name: string, data: Float32Array, size: number) => {
    const buffer = gl.createBuffer()
    if (!buffer) throw new Error('Voice buffer allocation failed')
    buffers.push(buffer)
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(gl.ARRAY_BUFFER, data, gl.STATIC_DRAW)
    const location = gl.getAttribLocation(program!, name)
    if (location < 0) return
    gl.enableVertexAttribArray(location)
    gl.vertexAttribPointer(location, size, gl.FLOAT, false, 0, 0)
  }
  try {
    if (!program) throw new Error('Voice renderer allocation failed')
    gl.attachShader(program, compile(gl.VERTEX_SHADER, particleVertex))
    gl.attachShader(program, compile(gl.FRAGMENT_SHADER, particleFragment))
    gl.linkProgram(program)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS))
      throw new Error(gl.getProgramInfoLog(program) || 'Voice shader linking failed')
    gl.useProgram(program)
    const data = particles()
    attribute('direction', data.directions, 3)
    attribute('seed', data.seeds, 1)
    attribute('lane', data.lanes, 1)
    gl.enable(gl.BLEND)
    const location = Object.fromEntries(
      uniformNames.map((name) => [name, gl.getUniformLocation(program, name)])
    ) as Record<(typeof uniformNames)[number], WebGLUniformLocation | null>
    return {
      draw(values: ParticleUniforms) {
        if (gl.isContextLost()) return
        gl.viewport(0, 0, canvas.width, canvas.height)
        gl.clearColor(0, 0, 0, 0)
        gl.clear(gl.COLOR_BUFFER_BIT)
        if (values.light) gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
        else gl.blendFunc(gl.ONE, gl.ONE)
        gl.uniform1f(location.time, values.time)
        gl.uniform1f(location.energy, values.energy)
        gl.uniform1f(location.bass, values.bass)
        gl.uniform1f(location.treble, values.treble)
        gl.uniform1f(location.swirl, values.look.swirl)
        gl.uniform1f(location.flare, values.look.flare)
        gl.uniform1f(location.ring, values.look.ring)
        gl.uniform1f(location.ripple, values.look.ripple)
        gl.uniform1f(location.dim, values.dim)
        gl.uniform1f(location.light, values.light)
        gl.uniform1f(location.pixel, canvas.width / 300)
        gl.uniform3fv(location.deep, values.look.deep)
        gl.uniform3fv(location.mid, values.look.mid)
        gl.uniform3fv(location.hot, values.look.hot)
        const band = (index: number) => values.bands[index] ?? 0
        gl.uniform4f(location.bandsA, band(0), band(3), band(6), band(9))
        gl.uniform4f(location.bandsB, band(1), band(4), band(7), band(10))
        gl.uniform4f(location.bandsC, band(2), band(5), band(8), band(11))
        gl.drawArrays(gl.POINTS, 0, count)
      },
      dispose
    }
  } catch (error) {
    console.error(error)
    dispose()
    return null
  }
}
