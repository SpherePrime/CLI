export function needsMaxReasoningNotice(next: string, current: string, skipNotice?: boolean) {
  return next === 'max' && current !== 'max' && !skipNotice
}
