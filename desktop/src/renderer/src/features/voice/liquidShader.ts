export const liquidVertex = `
attribute vec2 position;
varying vec2 uv;
void main() { uv = position; gl_Position = vec4(position, 0., 1.); }
`

export const liquidFragment = `
precision highp float;
varying vec2 uv;
uniform float time;
uniform float energy;
uniform float bass;
uniform float lightTheme;
uniform float opacity;

float ribbon(vec3 p) {
  float a = p.y * 2.8 + time * .19;
  p.xz = mat2(cos(a), -sin(a), sin(a), cos(a)) * p.xz;
  return sin(p.x * 4.2 + sin(p.y * 3.1 + time * .23) * 1.5)
    + cos(p.z * 3.8 - p.y * 1.8 + time * .17) * .72;
}

vec3 environment(vec3 r) {
  float strip = pow(max(0., dot(r, normalize(vec3(-.5, .8, 1.)))), 32.);
  float softbox = pow(max(0., dot(r, normalize(vec3(.8, .5, .6)))), 16.);
  float copper = pow(max(0., dot(r, normalize(vec3(-.8, -.4, .3)))), 9.);
  return vec3(.035, .042, .05) + vec3(.94, .95, 1.) * strip * 2.4
    + vec3(1., .89, .72) * softbox * 1.6 + vec3(1., .38, .12) * copper * .65;
}

void main() {
  vec2 p = uv * 1.19;
  float radius = .91 + energy * .035;
  float distance = dot(p, p);
  if (distance > radius * radius) { gl_FragColor = vec4(0.); return; }
  float depth = sqrt(radius * radius - distance);
  vec3 normal = normalize(vec3(p, depth));
  vec3 eye = vec3(0., 0., -1.);
  vec3 reflected = reflect(eye, normal);
  float fresnel = pow(1. - normal.z, 3.);
  vec3 color = mix(vec3(.025, .029, .034), vec3(.67, .63, .56), lightTheme);
  float transmission = 1.;
  vec3 warm = vec3(1., .55, .24);
  vec3 previousPoint = vec3(p * (1. - depth * .1), depth);
  float previousField = ribbon(previousPoint);
  for (int i = 0; i < 48; i++) {
    float z = depth * (1. - 2. * (float(i) + .5) / 48.);
    vec3 point = vec3(p * (1. - z * .1), z);
    float field = ribbon(point);
    if (field * previousField < 0.) {
      point = mix(previousPoint, point, previousField / (previousField - field));
      float gate = 1. - smoothstep(radius - .12, radius, length(point));
      float density = gate * .65;
      field = ribbon(point);
      vec2 e = vec2(.012, 0.);
      vec3 n = normalize(vec3(ribbon(point + e.xyy) - field,
        ribbon(point + e.yxy) - field, ribbon(point + e.yyx) - field));
      vec3 reflection = environment(reflect(eye, n));
      float glow = pow(max(0., sin(point.y * 3. + point.x * 2. - time * .6)), 10.);
      float edge = pow(1. - abs(n.z), 2.);
      vec3 material = vec3(.07, .075, .08) + reflection * 1.8;
      material += warm * (glow * (1. + energy * 2.2 + bass * .6) + edge * .65);
      color += material * density * transmission;
      transmission *= 1. - density * .72;
    }
    previousPoint = vec3(p * (1. - z * .1), z);
    previousField = ribbon(previousPoint);
  }
  color += environment(reflected) * (.16 + fresnel * .85);
  color += warm * pow(fresnel, 3.) * (.17 + energy * .22);
  float rim = pow(1. - normal.z, 15.);
  color += vec3(.9, .82, .68) * rim * .45;
  color = 1. - exp(-color * 1.6);
  float alpha = smoothstep(0., .016, radius * radius - distance) * opacity;
  gl_FragColor = vec4(color, alpha);
}
`
