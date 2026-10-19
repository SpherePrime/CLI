const report = /^(?:\x1b\[[?>]?[\d;]*[cRn]|\x1b\[[IO]|\x1b\][\d;]*[^\x07\x1b]*(?:\x07|\x1b\\))+$/
const deviceAttributes = /^(?:\x1b\[[?>][\d;]*c)+$/

export function isTerminalReport(data: string) {
  return data.length <= 256 && report.test(data)
}

export function isDeviceAttributesReply(data: string) {
  return deviceAttributes.test(data)
}

export const primaryDeviceAttributesQuery = /\x1b\[0?c/g
export const primaryDeviceAttributesReply = '\x1b[?61;4c'