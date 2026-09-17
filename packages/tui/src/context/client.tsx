import { createContext, useContext } from "solid-js"
import type { AgentClient } from "../client"

const ClientContext = createContext<{ client: () => AgentClient }>()

export function ClientProvider(props: { client: AgentClient; children: any }) {
  return (
    <ClientContext.Provider value={{ client: () => props.client }}>
      {props.children}
    </ClientContext.Provider>
  )
}

export function useClient() {
  const context = useContext(ClientContext)
  if (!context) throw new Error("useClient must be used within ClientProvider")
  return context.client()
}