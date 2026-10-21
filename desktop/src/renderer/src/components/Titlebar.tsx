import type { ReactNode } from 'react'
import { PrimeLogo } from './PrimeLogo'

export function Titlebar({ children }: { children?: ReactNode }) {
  return (
    <div className="titlebar">
      <div className="titlebar-brand">
        <PrimeLogo />
        <span>
          Prime<span className="brand-word-muted">Desktop</span>
        </span>
      </div>
      {children ? <div className="titlebar-content">{children}</div> : <div className="titlebar-drag" />}
      <div className="titlebar-window-space" />
    </div>
  )
}
