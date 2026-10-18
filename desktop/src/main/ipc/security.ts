export function validateExternalUrl(value: string): string {
  const url = new URL(value)
  if (!['https:', 'http:'].includes(url.protocol))
    throw new Error('Допустимы только HTTP и HTTPS ссылки')
  return url.href
}
