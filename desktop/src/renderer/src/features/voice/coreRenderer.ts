import { coreFragment, coreVertex } from './coreShader'
import type { CoreLook } from './coreStates'

export type CoreUniforms = {
  time: number
  energy: number
  bass: number
  treble: number
  dim: number
  light: number
  bands: number[]
  look: CoreLook
}

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
  'deep',
  'mid',
  'hot',
  'bandsA',
  'bandsB',
  'bandsC'
] as const

export function createCoreRenderer(canvas: HTMLCanvasElement) {
  const gl = canvas.getContext('webgl', {
    alpha: true,
    premultipliedAlpha: true,
    antialias: false,
    powerPreference: 'high-performance'
  })
  if (!gl) return null
  const shaders: WebGLShader[] = []
  const program = gl.createProgram()
  const buffer = gl.createBuffer()
  const dispose = () => {
    shaders.forEach((shader) => gl.deleteShader(shader))
    gl.deleteBuffer(buffer)
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
  try {
    if (!program || !buffer) throw new Error('Voice renderer allocation failed')
    gl.attachShader(program, compile(gl.VERTEX_SHADER, coreVertex))
    gl.attachShader(program, compile(gl.FRAGMENT_SHADER, coreFragment))
    gl.linkProgram(program)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS))
      throw new Error(gl.getProgramInfoLog(program) || 'Voice shader linking failed')
    gl.useProgram(program)
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]), gl.STATIC_DRAW)
    const position = gl.getAttribLocation(program, 'position')
    gl.enableVertexAttribArray(position)
    gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)
    const location = Object.fromEntries(uniformNames.map((name) => [name, gl.getUniformLocation(program, name)])) as Record<
      (typeof uniformNames)[number],
      WebGLUniformLocation | null
    >
    return {
      draw(values: CoreUniforms) {
        if (gl.isContextLost()) return
        gl.viewport(0, 0, canvas.width, canvas.height)
        gl.clearColor(0, 0, 0, 0)
        gl.clear(gl.COLOR_BUFFER_BIT)
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
        gl.uniform3fv(location.deep, values.look.deep)
        gl.uniform3fv(location.mid, values.look.mid)
        gl.uniform3fv(location.hot, values.look.hot)
        const band = (index: number) => values.bands[index] ?? 0
        gl.uniform4f(location.bandsA, band(0), band(3), band(6), band(9))
        gl.uniform4f(location.bandsB, band(1), band(4), band(7), band(10))
        gl.uniform4f(location.bandsC, band(2), band(5), band(8), band(11))
        gl.drawArrays(gl.TRIANGLES, 0, 6)
      },
      dispose
    }
  } catch (error) {
    console.error(error)
    dispose()
    return null
  }
}
