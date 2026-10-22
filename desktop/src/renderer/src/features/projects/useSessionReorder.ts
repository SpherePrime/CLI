import { useEffect, useRef, useState, type DragEvent, type KeyboardEvent, type MouseEvent } from 'react'
import { reorderProjectPaths } from './projectOrder'

type DropTarget = { projectId: string; id: string; position: 'before' | 'after' }
type DragSource = { projectId: string; id: string }

// Drag-and-drop reordering of chat rows. Each project reorders only its own
// chats: a drop is accepted from a source in the same project and the saved
// order is the reordered id list of that project.
export function useSessionReorder(
  idsOf: (projectId: string) => string[],
  saveOrder: (projectId: string, ids: string[]) => void
) {
  const source = useRef<DragSource | null>(null)
  const suppressClick = useRef(false)
  const [dragging, setDragging] = useState<DragSource | null>(null)
  const [target, setTarget] = useState<DropTarget | null>(null)
  const idsRef = useRef(idsOf)
  useEffect(() => {
    idsRef.current = idsOf
  })
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
    rowProps: (projectId: string, id: string) => ({
      'data-session-id': id,
      className: `session-reorder-row${dragging?.id === id ? ' session-dragging' : ''}${
        target?.id === id ? ` project-drop-${target.position}` : ''
      }`,
      onDragOver: (event: DragEvent<HTMLDivElement>) => {
        if (!source.current || source.current.projectId !== projectId || source.current.id === id)
          return
        event.preventDefault()
        event.dataTransfer.dropEffect = 'move'
        const position = positionAt(event)
        if (target?.id !== id || target.position !== position) setTarget({ projectId, id, position })
      },
      onDragLeave: (event: DragEvent<HTMLDivElement>) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setTarget(null)
      },
      onDrop: (event: DragEvent<HTMLDivElement>) => {
        if (!source.current) return
        event.preventDefault()
        event.stopPropagation()
        const ids = idsRef.current(projectId)
        saveOrder(
          projectId,
          reorderProjectPaths(ids, source.current.id, id, positionAt(event))
        )
        reset()
      }
    }),
    handleProps: (projectId: string, id: string) => ({
      draggable: true,
      'aria-keyshortcuts': 'Alt+ArrowUp Alt+ArrowDown',
      onDragStart: (event: DragEvent<HTMLElement>) => {
        source.current = { projectId, id }
        suppressClick.current = true
        event.dataTransfer.effectAllowed = 'move'
        event.dataTransfer.setData('text/plain', id)
        setDragging({ projectId, id })
      },
      onDragEnd: reset,
      onPointerDown: () => {
        suppressClick.current = false
      },
      onClickCapture: (event: MouseEvent<HTMLElement>) => {
        if (!suppressClick.current) return
        event.preventDefault()
        event.stopPropagation()
        suppressClick.current = false
      },
      onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
        if (!event.altKey || (event.key !== 'ArrowUp' && event.key !== 'ArrowDown')) return
        event.preventDefault()
        const ids = idsRef.current(projectId)
        const direction = event.key === 'ArrowUp' ? -1 : 1
        const adjacent = ids[ids.indexOf(id) + direction]
        if (adjacent)
          saveOrder(
            projectId,
            reorderProjectPaths(ids, id, adjacent, direction < 0 ? 'before' : 'after')
          )
      }
    })
  }
}
