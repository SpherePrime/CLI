import { createSignal } from "solid-js"

export type Route =
  | { type: "home" }
  | { type: "session"; sessionId?: string }

const [route, setRoute] = createSignal<Route>({ type: "home" })

export function useRoute() {
  return {
    route,
    navigate: setRoute,
  }
}

export function navigateTo(next: Route) {
  setRoute(next)
}
