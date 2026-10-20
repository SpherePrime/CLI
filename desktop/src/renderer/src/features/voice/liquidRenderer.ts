import { liquidFragment, liquidVertex } from './liquidShader'
import type { OrbFrame } from './orbRenderer'

export function isHolidayVoice(date = new Date()) {
  return (
    (date.getMonth() === 11 && date.getDate() >= 29) ||
    (date.getMonth() === 0 && date.getDate() <= 3)
  )
}

export function createLiquidRenderer() {
  const canvas = document.createElement('canvas')
  const gl = canvas.getContext('webgl', { alpha: true, premultipliedAlpha: false, antialias: true })
  if (!gl) return null
  const shaders: WebGLShader[] = []
  const compile = (type: number, source: string) => {
    const shader = gl.createShader(type)
    if (!shader) throw new Error('Unable to allocate voice shader')
    shaders.push(shader)
    gl.shaderSource(shader, source)
    gl.compileShader(shader)
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS))
      throw new Error(gl.getShaderInfoLog(shader) || 'Voice shader compilation failed')
    return shader
  }
  const program = gl.createProgram()
  const buffer = gl.createBuffer()
  const dispose = () => {
    shaders.forEach((shader) => gl.deleteShader(shader))
    gl.deleteBuffer(buffer)
    gl.deleteProgram(program)
    gl.getExtension('WEBGL_lose_context')?.loseContext()
  }
  try {
    if (!program || !buffer) throw new Error('Unable to allocate voice renderer')
    gl.attachShader(program, compile(gl.VERTEX_SHADER, liquidVertex))
    gl.attachShader(program, compile(gl.FRAGMENT_SHADER, liquidFragment))
    gl.linkProgram(program)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS))
      throw new Error('Voice shader linking failed')
    gl.useProgram(program)
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(
      gl.ARRAY_BUFFER,
      new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]),
      gl.STATIC_DRAW
    )
    const position = gl.getAttribLocation(program, 'position')
    gl.enableVertexAttribArray(position)
    gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)
    const uniforms = Object.fromEntries(
      ['time', 'energy', 'bass', 'lightTheme', 'opacity'].map((name) => [
        name,
        gl.getUniformLocation(program, name)
      ])
    )
    return {
      draw(context: CanvasRenderingContext2D, size: number, frame: OrbFrame, dark: boolean) {
        const resolution = Math.min(size, 720)
        if (canvas.width !== resolution) {
          canvas.width = resolution
          canvas.height = resolution
          gl.viewport(0, 0, resolution, resolution)
        }
        const active = !frame.muted || frame.state === 'speaking'
        gl.uniform1f(uniforms.time, frame.time)
        gl.uniform1f(uniforms.energy, active ? frame.level : 0)
        gl.uniform1f(uniforms.bass, active ? (frame.bands[0] ?? 0) : 0)
        gl.uniform1f(uniforms.lightTheme, dark ? 0 : 1)
        gl.uniform1f(uniforms.opacity, active ? 1 : 0.48)
        gl.drawArrays(gl.TRIANGLES, 0, 6)
        context.clearRect(0, 0, size, size)
        context.drawImage(canvas, 0, 0, size, size)
      },
      dispose
    }
  } catch {
    dispose()
    return null
  }
}
