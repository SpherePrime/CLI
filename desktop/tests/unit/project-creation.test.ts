import { describe, expect, it } from 'vitest'
import { projectNameError, projectCreationPath } from '../../src/shared/projectCreation'

describe('project creation', () => {
  it('accepts localized names and displays the native base path', () => {
    expect(projectNameError('Новый проект')).toBeNull()
    expect(projectCreationPath('C:\\Users\\me\\OneDrive\\Documents\\Prime', 'Новый проект')).toBe('C:\\Users\\me\\OneDrive\\Documents\\Prime\\Новый проект')
    expect(projectCreationPath('/home/me/Documents/Prime', 'demo')).toBe('/home/me/Documents/Prime/demo')
  })
  it.each(['', ' ', '.', '..', '../escape', 'a/b', 'a\\b', 'C:demo', 'a?', 'a\u0000', 'CON', 'nul.txt', 'COM1', 'LPT9.log', 'name.', 'name ', ' name', 'a'.repeat(81)])('rejects unsafe name %j', (name) => {
    expect(projectNameError(name)).not.toBeNull()
  })
})
