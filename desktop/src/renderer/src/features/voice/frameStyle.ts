export function sampledStyle(read: () => string, everyFrames = 30) {
  let value = ''
  let frames = 0
  return () => {
    if (frames++ % everyFrames === 0) value = read()
    return value
  }
}
