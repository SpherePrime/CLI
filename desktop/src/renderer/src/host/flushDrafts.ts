export async function flushDrafts() {
  const pending: Promise<unknown>[] = []
  window.dispatchEvent(new CustomEvent('prime:flush-drafts', { detail: pending }))
  await Promise.all(pending)
}
