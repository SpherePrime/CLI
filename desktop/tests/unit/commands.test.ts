import { expect, it } from 'vitest'
import { normalizeCommand, renderCommand } from '../../src/shared/commands'
it('normalizes Go command fields and substitutes literal argument values', () => {
  const command = normalizeCommand({
    Name: 'project:review',
    Content: 'Review $TARGET using $STYLE.',
    Arguments: [
      { ID: 'TARGET', Required: true },
      { ID: 'STYLE', Required: true }
    ]
  })
  expect(command.name).toBe('project:review')
  expect(renderCommand(command.content, { TARGET: 'src/$FILE', STYLE: 'strict' })).toBe(
    'Review src/$FILE using strict.'
  )
})
