import { describe, expect, it } from 'vitest'
import type { Message, Project } from '../../src/shared/contracts'
import {
  base64ToUint8Array,
  buildDesignImplementationPrompt,
  extractDesignFromMessage,
  filterRegularProjects,
  filterRegularRecentPaths,
  formatDesignPreviewText,
  isRegularProject
} from '../../src/renderer/src/features/designer/designHandoff'
import { previewDesignPngBase64 } from '../../src/renderer/src/host/previewHostData'

describe('designer handoff helpers', () => {
  it('filters out chat and designer workspaces from regular projects', () => {
    const projects: Project[] = [
      { id: '1', kind: 'chat', path: 'chat', clientId: 'c1', connection: 'ready' },
      { id: '2', kind: 'designer', path: 'C:\\Users\\demo\\Designer', clientId: 'c1', connection: 'ready' },
      { id: '3', kind: 'project', path: 'C:\\Users\\demo\\app', clientId: 'c1', connection: 'ready' },
      { id: '4', path: 'C:\\Users\\demo\\legacy', clientId: 'c1', connection: 'ready' }
    ]

    expect(isRegularProject(projects[0])).toBe(false)
    expect(isRegularProject(projects[1])).toBe(false)
    expect(isRegularProject(projects[2])).toBe(true)
    expect(isRegularProject(projects[3])).toBe(true)
    expect(filterRegularProjects(projects).map((item) => item.id)).toEqual(['3', '4'])
    expect(
      filterRegularRecentPaths(
        ['chat', 'C:\\Users\\demo\\Designer', 'C:\\Users\\demo\\app', 'C:\\Users\\demo\\closed'],
        projects
      )
    ).toEqual(['C:\\Users\\demo\\closed'])
  })

  it('extracts title, text and deduplicated images from tool calls, binary and image_url parts', () => {
    const pngA = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='
    const pngB = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII='

    const messages: Message[] = [
      {
        id: 'u1',
        session_id: 's1',
        role: 'user',
        parts: [{ type: 'text', data: { text: 'Сделай концепт панели мониторинга' } }]
      },
      {
        id: 'a1',
        session_id: 's1',
        role: 'assistant',
        parts: [
          {
            type: 'tool_call',
            data: {
              id: 'tc1',
              name: 'generate_image',
              result: {
                mime_type: 'image/png',
                data: pngA,
                images: [
                  { mime_type: 'image/png', data: pngA, path: 'hero.png' },
                  { mime_type: 'image/png', data: pngB, path: 'mobile.png' }
                ]
              }
            }
          }
        ]
      },
      {
        id: 'a2',
        session_id: 's1',
        role: 'assistant',
        parts: [
          {
            type: 'text',
            data: {
              text: '### Дашборд аналитики\n\nИспользуйте тёплую тёмную палитру и акцент `#ff834f`.'
            }
          },
          {
            type: 'image_url',
            data: { url: `data:image/png;base64,${pngA}` }
          },
          {
            type: 'binary',
            data: { mime_type: 'image/webp', data: 'UklGRg==', path: 'extra.webp' }
          }
        ]
      }
    ]

    const extracted = extractDesignFromMessage(messages[2], messages)
    expect(extracted.title).toBe('Дашборд аналитики')
    expect(extracted.text).toContain('Используйте тёплую тёмную палитру')
    expect(extracted.images).toHaveLength(3)
    expect(extracted.images.map((item) => item.name)).toEqual([
      'design-mockup-1.png',
      'mobile.png',
      'extra.webp'
    ])
    expect(base64ToUint8Array(pngA).slice(0, 4)).toEqual(new Uint8Array([137, 80, 78, 71]))
  })

  it('builds a structured implementation prompt with image attachment context', () => {
    const promptWithImages = buildDesignImplementationPrompt({
      title: 'Лендинг',
      text: 'Минималистичный hero-экран с контрастной кнопкой.',
      images: [{ name: 'hero.png', mimeType: 'image/png', base64: 'AAA=' }]
    })
    expect(promptWithImages).toContain('Реализуй в этом проекте утверждённый дизайн')
    expect(promptWithImages).toContain('прикреплён утверждённый визуальный макет')
    expect(promptWithImages).toContain('Минималистичный hero-экран')

    const promptTextOnly = buildDesignImplementationPrompt({
      title: 'Лендинг',
      text: 'Только текстовая спецификация',
      images: []
    })
    expect(promptTextOnly).not.toContain('прикреплён')
    expect(promptTextOnly).toContain('Только текстовая спецификация')
  })

  it('strips markdown markers for handoff preview snippets and builds a valid 160x100 preview PNG', () => {
    const snippet = formatDesignPreviewText(
      '### Концепт главного экрана\n\n- **Палитра:** фон `#151618` и акцент `#ff834f`.\n> Подсказка'
    )
    expect(snippet).toBe('Концепт главного экрана · Палитра: фон #151618 и акцент #ff834f. · Подсказка')
    expect(snippet).not.toContain('###')
    expect(snippet).not.toContain('**')
    expect(snippet).not.toContain('`')

    const bytes = base64ToUint8Array(previewDesignPngBase64)
    expect(bytes.slice(0, 8)).toEqual(new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]))
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
    expect(view.getUint32(16, false)).toBe(160)
    expect(view.getUint32(20, false)).toBe(100)
  })
})
