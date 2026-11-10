export const nativeDragExclusionSelector =
  'button,a,input,select,textarea,[role="button"],.breadcrumbs,.native-window-controls,.titlebar-window-space'

export function shouldIgnoreNativeDrag(target: Element | null, button: number): boolean {
  if (button !== 0 || !target) return true
  return Boolean(target.closest(nativeDragExclusionSelector))
}
