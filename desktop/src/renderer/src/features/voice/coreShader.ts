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
uniform vec4 bandsA;
uniform vec4 bandsB;
uniform vec4 bandsC;

const float radius = 0.5;
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

float smin(float a, float b, float k) {
  float h = clamp(0.5 + 0.5 * (b - a) / k, 0.0, 1.0);
  return mix(b, a, h) - k * h * (1.0 - h);
}

float lobe(float angle, float center, float value) {
  float c = max(0.0, cos(angle - center));
  c *= c;
  c *= c;
  return value * c * c;
}

float voice(vec3 dir, float t) {
  float angle = atan(dir.y, dir.x) + t * swirl * 0.25;
  float step = tau / 12.0;
  float shape = 0.0;
  shape += lobe(angle, step * 0.0, bandsA.x);
  shape += lobe(angle, step * 1.0, bandsB.x);
  shape += lobe(angle, step * 2.0, bandsC.x);
  shape += lobe(angle, step * 3.0, bandsA.y);
  shape += lobe(angle, step * 4.0, bandsB.y);
  shape += lobe(angle, step * 5.0, bandsC.y);
  shape += lobe(angle, step * 6.0, bandsA.z);
  shape += lobe(angle, step * 7.0, bandsB.z);
  shape += lobe(angle, step * 8.0, bandsC.z);
  shape += lobe(angle, step * 9.0, bandsA.w);
  shape += lobe(angle, step * 10.0, bandsB.w);
  shape += lobe(angle, step * 11.0, bandsC.w);
  float equator = 1.0 - dir.z * dir.z;
  return shape * equator * sqrt(equator);
}

float scene(vec3 p, float t) {
  vec3 dir = normalize(p + vec3(0.0, 0.0, 0.0001));
  float calm = 0.045 + flare * 0.03;
  float flow = noise(p * 2.4 + vec3(0.0, t * 0.45, t * 0.3)) * (calm + energy * 0.07);
  flow += noise(p * 5.5 - vec3(t * 0.6, 0.0, t * 0.4)) * (0.012 + treble * 0.05);
  float waves = ripple * sin(dot(dir, vec3(0.6, 0.8, 0.0)) * 9.0 + length(p.xy) * 14.0 - t * 7.0) * (0.012 + energy * 0.05);
  float size = radius * (1.0 + bass * 0.12 + sin(t * 1.4) * 0.012);
  float body = length(p) - size * (1.0 + flow + waves + voice(dir, t) * (0.2 + flare * 0.16));
  if (ring > 0.01) {
    for (int i = 0; i < 3; i++) {
      float a = t * 1.3 + float(i) * tau / 3.0;
      float orbit = size * (1.25 + 0.35 * sin(t * 0.9 + float(i) * 2.1));
      vec3 center = vec3(cos(a) * orbit, sin(a * 1.3) * size * 0.35, sin(a) * orbit * 0.6);
      float drop = length(p - center) - size * 0.24 * ring;
      body = smin(body, drop, 0.11 * ring + 0.001);
    }
  }
  return body;
}

vec3 normalAt(vec3 p, float t) {
  vec2 e = vec2(0.0025, 0.0);
  return normalize(vec3(
    scene(p + e.xyy, t) - scene(p - e.xyy, t),
    scene(p + e.yxy, t) - scene(p - e.yxy, t),
    scene(p + e.yyx, t) - scene(p - e.yyx, t)
  ));
}

void main() {
  float t = time;
  vec2 view = uv * 1.12;
  vec3 ro = vec3(view, -2.0);
  vec3 rd = vec3(0.0, 0.0, 1.0);
  float travelled = 0.0;
  float nearest = 10.0;
  float hit = 0.0;
  vec3 p = ro;
  if (length(view) < 1.08) {
    for (int i = 0; i < 44; i++) {
      p = ro + rd * travelled;
      float d = scene(p, t);
      nearest = min(nearest, d);
      if (d < 0.0015) {
        hit = 1.0;
        break;
      }
      travelled += d * 0.75;
      if (travelled > 4.0) break;
    }
  }

  vec3 color = vec3(0.0);
  if (hit > 0.5) {
    vec3 n = normalAt(p, t);
    vec3 v = -rd;
    float facing = clamp(dot(n, v), 0.0, 1.0);
    float fresnel = pow(1.0 - facing, 3.0);
    float height = n.y * 0.5 + 0.5;
    float flowShade = noise(p * 3.0 + vec3(t * 0.25, -t * 0.2, 0.0)) * 0.5 + 0.5;
    float depthTone = smoothstep(0.0, 1.0, facing);
    vec3 base = mix(deep * 0.8, mid, smoothstep(0.05, 0.9, depthTone * 0.75 + height * 0.2 + flowShade * 0.2));
    float caustic = pow(0.5 + 0.5 * sin(dot(n, vec3(4.0, 6.0, 2.0)) + flowShade * 5.0 + t * 1.1), 6.0);
    vec3 inner = mix(mid, hot, pow(facing, 1.6)) * (0.6 + energy * 0.7) + hot * caustic * 0.35 * facing;
    color = mix(base, inner, smoothstep(0.2, 1.0, facing) * 0.72);
    float iridescence = 0.5 + 0.5 * sin(fresnel * 7.0 + t * 0.8 + n.x * 3.0 + n.y * 2.0);
    vec3 sheen = mix(vec3(1.0, 0.55, 0.75), vec3(0.6, 0.7, 1.0), iridescence);
    color += mix(hot, sheen, 0.55) * fresnel * 1.2;
    vec3 key = normalize(vec3(-0.5, 0.7, -0.6));
    vec3 fill = normalize(vec3(0.7, -0.35, -0.5));
    vec3 reflected = reflect(rd, n);
    color += vec3(1.0, 0.97, 0.94) * pow(max(dot(reflected, key), 0.0), 40.0) * 0.9;
    color += hot * pow(max(dot(reflected, fill), 0.0), 12.0) * 0.35;
    color += hot * pow(max(0.0, 1.0 - facing), 1.5) * energy * 0.35;
  }

  float bound = smoothstep(1.08, 0.82, length(view));
  float halo = exp(-max(nearest, 0.0) * (11.0 - energy * 3.0)) * (1.0 - hit) * (0.32 + energy * 0.5 + bass * 0.25) * bound;
  vec3 glow = mix(mid, hot, 0.35) * halo;

  float luminance = dot(color, vec3(0.299, 0.587, 0.114));
  color = mix(color, vec3(luminance) * vec3(0.95, 0.93, 0.92), dim * 0.8) * (1.0 - dim * 0.5);
  glow *= 1.0 - dim * 0.75;
  color = 1.0 - exp(-color * 1.25);
  glow = 1.0 - exp(-glow * 1.3);
  vec3 outColor = color * hit + glow;
  float alpha = max(hit, max(glow.r, max(glow.g, glow.b)));
  gl_FragColor = vec4(outColor, alpha);
}
`
