export const DEFAULT_VIEWPORTS = [
  { name: "mobile-320", width: 320, height: 640, isMobile: true, hasTouch: true },
  { name: "mobile-390", width: 390, height: 844, isMobile: true, hasTouch: true },
  { name: "tablet-768", width: 768, height: 1024, isMobile: true, hasTouch: true },
  { name: "laptop-1280", width: 1280, height: 800 },
  { name: "desktop-1440", width: 1440, height: 900 },
  { name: "wide-1920", width: 1920, height: 1080 },
];

export function parseViewports(value) {
  if (!value) return DEFAULT_VIEWPORTS;
  return value.split(",").map((entry) => {
    const [width, height] = entry.trim().split("x").map(Number);
    if (!width || !height) throw new Error(`Bad viewport "${entry}", expected WIDTHxHEIGHT`);
    const isMobile = width < 1024;
    return { name: `custom-${width}x${height}`, width, height, isMobile, hasTouch: isMobile };
  });
}
