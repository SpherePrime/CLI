import { describe, expect, it } from 'vitest'
import { toolTitle } from '../../src/renderer/src/features/chat/toolTitle'

const t = (value: string) => value

describe('goal_complete title', () => {
  it('shows a readable label with the summary', () => {
    const title = toolTitle('goal_complete', { summary: 'Site   delivered and checked' }, t)
    expect(title.label).toBe('Цель выполнена')
    expect(title.detail).toBe('Site delivered and checked')
  })
})
