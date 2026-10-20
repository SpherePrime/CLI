export const coreVertex = `
attribute vec2 position;
varying vec2 uv;
void main() {
  uv = position;
  gl_Position = vec4(position, 0.0, 1.0);
}
`

export const coreFragment = `
precision highp float;
varying vec2 uv;

uniform float time;
uniform float energy;
uniform float bass;
uniform float treble;
uniform float swirl;
uniform float flare;
uniform float ring;
uniform float ripple;
uniform float dim;
uniform float light;
uniform vec3 deep;
uniform vec3 mid;
uniform vec3 hot;

const float radius = 0.5;

vec3 mod289(vec3 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
vec4 mod289(vec4 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
vec4 permute(vec4 x) { return mod289(((x * 34.0) + 1.0) * x); }
vec4 taylorInvSqrt(vec4 r) { return 1.79284291400159 - 0.85373472095314 * r; }

float snoise(vec3 v) {
  const vec2 C = vec2(1.0 / 6.0, 1.0 / 3.0);
  const vec4 D = vec4(0.0, 0.5, 1.0, 2.0);
  vec3 i = floor(v + dot(v, C.yyy));
  vec3 x0 = v - i + dot(i, C.xxx);
  vec3 g = step(x0.yzx, x0.xyz);
  vec3 l = 1.0 - g;
  vec3 i1 = min(g.xyz, l.zxy);
  vec3 i2 = max(g.xyz, l.zxy);
  vec3 x1 = x0 - i1 + C.xxx;
  vec3 x2 = x0 - i2 + C.yyy;
  vec3 x3 = x0 - D.yyy;
  i = mod289(i);
  vec4 p = permute(permute(permute(i.z + vec4(0.0, i1.z, i2.z, 1.0)) + i.y + vec4(0.0, i1.y, i2.y, 1.0)) + i.x + vec4(0.0, i1.x, i2.x, 1.0));
  float n_ = 0.142857142857;
  vec3 ns = n_ * D.wyz - D.xzx;
  vec4 j = p - 49.0 * floor(p * ns.z * ns.z);
  vec4 x_ = floor(j * ns.z);
  vec4 y_ = floor(j - 7.0 * x_);
  vec4 x = x_ * ns.x + ns.yyyy;
  vec4 y = y_ * ns.x + ns.yyyy;
  vec4 h = 1.0 - abs(x) - abs(y);
  vec4 b0 = vec4(x.xy, y.xy);
  vec4 b1 = vec4(x.zw, y.zw);
  vec4 s0 = floor(b0) * 2.0 + 1.0;
  vec4 s1 = floor(b1) * 2.0 + 1.0;
  vec4 sh = -step(h, vec4(0.0));
  vec4 a0 = b0.xzyw + s0.xzyw * sh.xxyy;
  vec4 a1 = b1.xzyw + s1.xzyw * sh.zzww;
  vec3 p0 = vec3(a0.xy, h.x);
  vec3 p1 = vec3(a0.zw, h.y);
  vec3 p2 = vec3(a1.xy, h.z);
  vec3 p3 = vec3(a1.zw, h.w);
  vec4 norm = taylorInvSqrt(vec4(dot(p0, p0), dot(p1, p1), dot(p2, p2), dot(p3, p3)));
  p0 *= norm.x; p1 *= norm.y; p2 *= norm.z; p3 *= norm.w;
  vec4 m = max(0.6 - vec4(dot(x0, x0), dot(x1, x1), dot(x2, x2), dot(x3, x3)), 0.0);
  m = m * m;
  return 42.0 * dot(m * m, vec4(dot(p0, x0), dot(p1, x1), dot(p2, x2), dot(p3, x3)));
}

float fbm(vec3 p) {
  float value = 0.0;
  float amplitude = 0.5;
  for (int i = 0; i < 3; i++) {
    value += amplitude * snoise(p);
    p = p * 2.03 + vec3(1.7, 9.2, 3.1);
    amplitude *= 0.5;
  }
  return value;
}

mat2 rotate(float a) {
  float c = cos(a);
  float s = sin(a);
  return mat2(c, -s, s, c);
}

vec3 palette(float t) {
  t = clamp(t, 0.0, 1.0);
  vec3 color = mix(deep, mid, smoothstep(0.0, 0.55, t));
  color = mix(color, hot, smoothstep(0.45, 0.9, t));
  return mix(color, vec3(1.0, 0.96, 0.9), smoothstep(0.93, 1.0, t) * 0.6);
}

vec3 field(vec3 p, float t) {
  p.xz *= rotate(t * swirl * 0.35);
  p.xy *= rotate(0.35 + sin(t * 0.21) * 0.15);
  vec3 warp = vec3(
    snoise(p * 1.4 + vec3(0.0, 0.0, t * 0.17)),
    snoise(p * 1.4 + vec3(5.2, 1.3, -t * 0.13)),
    snoise(p * 1.4 + vec3(9.1, 4.7, t * 0.11))
  );
  vec3 q = p + warp * (0.38 + energy * 0.45 + bass * 0.2);
  float n = fbm(q * 1.25 + vec3(0.0, t * 0.2, 0.0));
  float veins = pow(1.0 - abs(n), 3.2);
  float strands = pow(0.5 + 0.5 * sin(q.y * 5.0 + n * 4.0 + t * 0.6), 6.0);
  float core = exp(-dot(p, p) * 4.2);
  float waves = ripple * pow(0.5 + 0.5 * sin(length(p) * 11.0 - t * 6.0), 3.0) * (0.3 + energy * 1.4);
  veins = min(veins * (1.0 + waves * 1.1), 1.15);
  float density = veins * (0.8 + treble * 0.5) + strands * 0.3 + core * (0.4 + energy * 0.35);
  float heat = clamp(veins * 0.62 + strands * 0.18 + core * 0.38 + energy * 0.1, 0.0, 1.0);
  return vec3(density, heat, veins);
}

float halo(float r, float a, float t) {
  float ray = fbm(vec3(cos(a) * 1.6, sin(a) * 1.6, t * 0.35)) * 0.5 + 0.5;
  float spikes = pow(ray, 3.0) * (flare + energy * 1.8 + bass * 1.1);
  float distance = max(0.0, r - radius);
  float aura = exp(-distance * 9.0) * (0.22 + energy * 0.35);
  return aura + exp(-distance * (12.0 - spikes * 7.0)) * spikes * 0.6;
}

void main() {
  vec2 p = uv;
  float r = length(p);
  float a = atan(p.y, p.x);
  float t = time;
  vec3 body = vec3(0.0);
  vec3 glow = vec3(0.0);
  float edge = 0.0;
  float pulse = 1.0 + energy * 0.06 + bass * 0.04 + sin(t * 1.3) * 0.008;
  float sphere = radius * pulse;

  if (r < sphere) {
    float depth = sqrt(sphere * sphere - r * r);
    vec3 normal = normalize(vec3(p, depth));
    float transmission = 1.0;
    vec3 inner = vec3(0.0);
    for (int i = 0; i < 10; i++) {
      float z = depth * (1.0 - 2.0 * (float(i) + 0.5) / 10.0);
      vec3 point = vec3(p, z) / sphere;
      vec3 cell = field(point, t);
      float density = cell.x * 0.14;
      inner += palette(cell.y) * density * transmission * (1.0 + cell.z * 0.6);
      transmission *= 1.0 - clamp(density * 0.55, 0.0, 0.9);
    }
    float fresnel = pow(1.0 - normal.z, 2.6);
    vec3 glass = mix(vec3(0.02, 0.018, 0.025), vec3(0.92, 0.9, 0.88), light);
    float limb = mix(0.35, 1.0, pow(normal.z, 0.8));
    float coreGlow = exp(-r * r / (sphere * sphere) * 3.0);
    body = glass * transmission + inner * 1.25 * limb + mid * coreGlow * (0.12 + energy * 0.25);
    float iridescence = 0.5 + 0.5 * sin(a * 2.0 + t * 0.6 + normal.z * 6.0);
    body += mix(mid, hot, iridescence) * fresnel * (0.7 + energy * 0.5);
    vec3 lightDir = normalize(vec3(-0.45, 0.6, 0.65));
    float spec = pow(max(dot(reflect(vec3(0.0, 0.0, -1.0), normal), lightDir), 0.0), 48.0);
    body += vec3(1.0) * spec * 0.55;
    edge = smoothstep(0.0, 0.006, sphere - r);
  }

  float corona = halo(r, a, t) * smoothstep(sphere - 0.03, sphere + 0.01, r);
  glow += palette(0.55 + energy * 0.3) * corona;

  if (ring > 0.001) {
    vec2 rp = p;
    rp.y /= 0.32;
    float band = abs(length(rp) - 0.78);
    float sweep = 0.5 + 0.5 * cos(atan(rp.y, rp.x) - t * 2.4);
    float front = step(p.y, 0.0) + step(0.0, p.y) * step(sphere, r);
    float glowRing = exp(-band * 70.0) * (0.25 + pow(sweep, 6.0) * 1.4) * ring * min(front, 1.0);
    glow += palette(0.8) * glowRing;
  }

  vec3 color = body * edge;
  float luminance = dot(color, vec3(0.299, 0.587, 0.114));
  color = mix(color, vec3(luminance) * vec3(0.95, 0.92, 0.9), dim * 0.75) * (1.0 - dim * 0.45);
  float glowLuminance = dot(glow, vec3(0.299, 0.587, 0.114));
  glow = mix(glow, vec3(glowLuminance), dim * 0.75) * (1.0 - dim * 0.6);
  color = 1.0 - exp(-color * 1.15);
  glow = 1.0 - exp(-glow * 1.2);
  vec3 outColor = color + glow * (1.0 - edge * 0.6);
  float alpha = max(edge, max(glow.r, max(glow.g, glow.b)) * (1.0 - edge));  gl_FragColor = vec4(outColor, alpha);
}
`
