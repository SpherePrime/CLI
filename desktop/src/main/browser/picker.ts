import type { WebContents } from 'electron'
import { pickScript } from './pickerScript'

export type PickedElement = {
  selector: string
  tag: string
  text: string
  html: string
  url: string
  rect: { x: number; y: number; width: number; height: number }
  styles: Record<string, string>
}

export type PickedCapture = { element: PickedElement; image?: Buffer }

async function capture(contents: WebContents, element: PickedElement): Promise<PickedCapture> {
  const zoom = contents.getZoomFactor()
  const rect = {
    x: Math.max(0, Math.floor(element.rect.x * zoom)),
    y: Math.max(0, Math.floor(element.rect.y * zoom)),
    width: Math.max(1, Math.ceil(element.rect.width * zoom)),
    height: Math.max(1, Math.ceil(element.rect.height * zoom))
  }
  try {
    const image = await contents.capturePage(rect)
    return { element, image: image.isEmpty() ? undefined : image.toPNG() }
  } catch {
    return { element }
  }
}

export async function pickElements(contents: WebContents): Promise<PickedCapture[] | null> {
  contents.focus()
  let stop = () => {}
  const navigated = new Promise<null>((resolve) => {
    const done = () => resolve(null)
    contents.once('did-start-navigation', done)
    contents.once('destroyed', done)
    stop = () => {
      contents.removeListener('did-start-navigation', done)
      contents.removeListener('destroyed', done)
    }
  })
  const elements = (await Promise.race([
    contents.executeJavaScript(pickScript, true) as Promise<PickedElement[] | null>,
    navigated
  ]).finally(stop)) as PickedElement[] | null
  if (!elements?.length) return null
  await new Promise((resolve) => setTimeout(resolve, 50))
  const captures: PickedCapture[] = []
  for (const element of elements) captures.push(await capture(contents, element))
  return captures
}

export function cancelPick(contents: WebContents) {
  return contents
    .executeJavaScript('window.__primePicker && window.__primePicker.cancel()', true)
    .catch(() => {})
}
