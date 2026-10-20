export const particleVertex = `
precision highp float;
attribute vec3 direction;
attribute float seed;
attribute float lane;

uniform float time;
uniform float energy;
uniform float bass;
uniform float treble;
uniform float swirl;
uniform float flare;
uniform float ring;
uniform float ripple;
uniform float dim;
uniform float pixel;
uniform vec4 bandsA;
uniform vec4 bandsB;
uniform vec4 bandsC;
uniform vec3 deep;
uniform vec3 mid;
uniform vec3 hot;

varying vec3 tint;
varying float glow;

const float tau = 6.2831853;

float hash(vec3 p) {
  p = fract(p * 0.3183099 + 0.1);
  p *= 17.0;
  return fract(p.x * p.y * p.z * (p.x + p.y + p.z));
}

float noise(vec3 x) {
  vec3 i = floor(x);
  vec3 f = fract(x);
  f = f * f * (3.0 - 2.0 * f);
  return mix(
    mix(mix(hash(i), hash(i + vec3(1.0, 0.0, 0.0)), f.x), mix(hash(i + vec3(0.0, 1.0, 0.0)), hash(i + vec3(1.0, 1.0, 0.0)), f.x), f.y),
    mix(mix(hash(i + vec3(0.0, 0.0, 1.0)), hash(i + vec3(1.0, 0.0, 1.0)), f.x), mix(hash(i + vec3(0.0, 1.0, 1.0)), hash(i + vec3(1.0, 1.0, 1.0)), f.x), f.y),
    f.z
  ) * 2.0 - 1.0;
}

mat2 rotate(float a) {
  float c = cos(a);
  float s = sin(a);
  return mat2(c, -s, s, c);
}

float lobe(float angle, float center, float value) {
  float c = max(0.0, cos(angle - center));
  c *= c;
  c *= c;
  return value * c * c;
}

float voice(float angle) {
  float step = tau / 12.0;
  return lobe(angle, 0.0, bandsA.x) + lobe(angle, step, bandsB.x) + lobe(angle, step * 2.0, bandsC.x)
    + lobe(angle, step * 3.0, bandsA.y) + lobe(angle, step * 4.0, bandsB.y) + lobe(angle, step * 5.0, bandsC.y)
    + lobe(angle, step * 6.0, bandsA.z) + lobe(angle, step * 7.0, bandsB.z) + lobe(angle, step * 8.0, bandsC.z)
    + lobe(angle, step * 9.0, bandsA.w) + lobe(angle, step * 10.0, bandsB.w) + lobe(angle, step * 11.0, bandsC.w);
}

vec3 sphereShape(vec3 dir, float t) {
  vec3 jitter = vec3(fract(seed * 12.9898), fract(seed * 78.233), fract(seed * 37.719)) - 0.5;
  vec3 d = normalize(dir + jitter * 0.035);
  d.xz *= rotate(t * (0.12 + swirl * 0.1));
  d.yz *= rotate(0.32);
  float angle = atan(d.y, d.x);
  float equator = 1.0 - d.z * d.z;
  float flow = noise(d * 2.3 + vec3(0.0, t * 0.35, t * 0.25));
  float shimmer = noise(d * 6.0 - vec3(t * 0.5)) * (0.01 + treble * 0.05);
  float spoken = voice(angle + t * 0.15) * equator * (0.22 + flare * 0.16);
  float waves = ripple * sin(d.y * 11.0 - t * 6.5 + seed * 0.6) * (0.015 + energy * 0.09);
  float radius = 0.52 * (1.0 + bass * 0.12 + sin(t * 1.3) * 0.012) * (1.0 - dim * 0.22);
  float spark = step(0.94, seed) * (0.12 + energy * 0.45 + bass * 0.2) * fract(seed * 13.7 + t * (0.1 + energy * 0.3));
  return d * radius * (1.0 + flow * (0.05 + energy * 0.1) + shimmer + spoken + waves + spark);
}

vec3 ringShape(float t) {
  float angle = fract(seed * 7.31 + direction.x * 0.5) * tau + t * (0.7 + lane * 0.18);
  float radius = 0.66 + lane * 0.07;
  vec3 p = vec3(cos(angle) * radius, sin(angle) * radius, 0.0) + direction * 0.022;
  p.yz *= rotate(0.55 + lane * 0.38);
  p.xy *= rotate(lane * 2.094 + t * 0.15);
  return p;
}

void main() {
  float t = time;
  float keep = step(seed, 0.32);
  vec3 sphere = sphereShape(direction, t) * (1.0 - ring * keep * 0.55);
  float morph = smoothstep(seed * 0.5, seed * 0.5 + 0.5, ring) * (1.0 - keep);
  vec3 position = mix(sphere, ringShape(t), morph);
  float depth = position.z + 2.6;
  vec2 screen = position.xy * 2.15 / depth;
  gl_Position = vec4(screen, 0.0, 1.0);
  float near = clamp(1.25 - position.z * 0.9, 0.25, 1.6);
  gl_PointSize = pixel * (1.1 + seed * 1.4 + energy * 1.2) * near / depth * 2.6;
  float heat = clamp(0.35 + direction.y * 0.25 + energy * 0.55 + step(0.9, seed) * 0.35, 0.0, 1.0);
  tint = mix(mix(deep, mid, smoothstep(0.0, 0.55, heat)), hot, smoothstep(0.5, 1.0, heat));
  tint = mix(tint, vec3(dot(tint, vec3(0.299, 0.587, 0.114))) * vec3(0.95, 0.94, 1.0), dim * 0.85);
  float sparkFade = 1.0 - step(0.94, seed) * fract(seed * 13.7 + t * (0.1 + energy * 0.3)) * (1.0 - morph);
  glow = near * (0.55 + energy * 0.6) * (1.0 - dim * 0.65) * sparkFade;
}
`

export const particleFragment = `
precision highp float;
varying vec3 tint;
varying float glow;
uniform float light;

void main() {
  vec2 p = gl_PointCoord * 2.0 - 1.0;
  float r = dot(p, p);
  if (r > 1.0) discard;
  float core = exp(-r * 4.5);
  float alpha = core * glow;
  vec3 color = mix(tint, vec3(1.0), core * core * 0.35);
  if (light > 0.5) color *= 0.8;
  gl_FragColor = vec4(color * alpha, alpha);
}
`
