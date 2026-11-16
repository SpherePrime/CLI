import { createContext, useContext, type ReactNode } from 'react'

const WorkspaceMediaContext = createContext('')

export function WorkspaceMediaProvider({
  workspaceId,
  children
}: {
  workspaceId: string
  children: ReactNode
}) {
  return (
    <WorkspaceMediaContext.Provider value={workspaceId}>
      {children}
    </WorkspaceMediaContext.Provider>
  )
}

export function useWorkspaceMediaId() {
  return useContext(WorkspaceMediaContext)
}
