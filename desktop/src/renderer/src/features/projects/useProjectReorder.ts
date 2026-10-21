import { useRef, useState, type DragEvent, type KeyboardEvent, type MouseEvent } from 'react'
import { reorderProjectPaths } from './projectOrder'

type DropTarget = { path: string; position: 'before' | 'after' }

export function useProjectReorder(paths: string[], saveOrder: (paths: string[]) => void) {
  const source = useRef<string | null>(null)
  const suppressClick = useRef(false)
  const [dragging, setDragging] = useState<string | null>(null)
  const [target, setTarget] = useState<DropTarget | null>(null)
  const reset = () => {
    source.current = null
    setDragging(null)
    setTarget(null)
  }
  const positionAt = (event: DragEvent<HTMLElement>): DropTarget['position'] => {
    const bounds = event.currentTarget.getBoundingClientRect()
    return event.clientY < bounds.top + bounds.height / 2 ? 'before' : 'after'
  }
  return {
    headingProps: (path: string) => ({
      'data-project-path': path,
      className: `project-heading project-reorder-heading${dragging === path ? ' project-dragging' : ''}${target?.path === path ? ` project-drop-${target.position}` : ''}`,
      onDragOver: (event: DragEvent<HTMLDivElement>) => {
        if (!source.current || source.current === path) return
        event.preventDefault()
        event.dataTransfer.dropEffect = 'move'
        const position = positionAt(event)
        if (target?.path !== path || target.position !== position) setTarget({ path, position })
      },
      onDragLeave: (event: DragEvent<HTMLDivElement>) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setTarget(null)
      },
      onDrop: (event: DragEvent<HTMLDivElement>) => {
        if (!source.current) return
        event.preventDefault()
        event.stopPropagation()
        saveOrder(reorderProjectPaths(paths, source.current, path, positionAt(event)))
        reset()
      }
    }),
    buttonProps: (path: string) => ({
      draggable: true,
      'aria-keyshortcuts': 'Alt+ArrowUp Alt+ArrowDown',
      onDragStart: (event: DragEvent<HTMLButtonElement>) => {
        source.current = path
        suppressClick.current = true
        event.dataTransfer.effectAllowed = 'move'
        event.dataTransfer.setData('text/plain', path)
        setDragging(path)
      },
      onDragEnd: reset,
      onPointerDown: () => {
        suppressClick.current = false
      },
      onClickCapture: (event: MouseEvent<HTMLButtonElement>) => {
        if (!suppressClick.current) return
        event.preventDefault()
        event.stopPropagation()
        suppressClick.current = false
      },
      onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => {
        if (!event.altKey || (event.key !== 'ArrowUp' && event.key !== 'ArrowDown')) return
        event.preventDefault()
        const direction = event.key === 'ArrowUp' ? -1 : 1
        const adjacent = paths[paths.indexOf(path) + direction]
        if (adjacent)
          saveOrder(reorderProjectPaths(paths, path, adjacent, direction < 0 ? 'before' : 'after'))
      }
    })
  }
}
