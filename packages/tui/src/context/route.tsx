import { createContext, useContext, createSignal } from "solid-js"

export type Route =
  | { type: "home" }
  | { type: "session"; sessionId?: string }

const RouteContext = createContext<{
  route: () => Route
  navigate: (route: Route) => void
}>()

export function RouteProvider(props: {
  initialRoute: Route
  children: any
}) {
  const [route, setRoute] = createSignal<Route>(props.initialRoute)
  return (
    <RouteContext.Provider
      value={{
        route,
        navigate: setRoute,
      }}
    >
      {props.children}
    </RouteContext.Provider>
  )
}

export function useRoute() {
  const context = useContext(RouteContext)
  if (!context) throw new Error("useRoute must be used within RouteProvider")
  return context
}