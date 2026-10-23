import type { ReactNode } from 'react'
import { PrimeLogo } from './PrimeLogo'
import { NativeWindowControls, dragNativeWindow } from '../host/windowChrome'

export function Titlebar({ children }: { children?: ReactNode }) {
  return (
    <div
      className="titlebar"
      onMouseDown={import.meta.env.VITE_NATIVE_HOST === 'true' ? dragNativeWindow : undefined}
    >
      <div className="titlebar-brand">
        <PrimeLogo />
        <span>
          Prime<span className="brand-word-muted">Desktop</span>
        </span>
      </div>
      {children ? (
        <div className="titlebar-content">{children}</div>
      ) : (
        <div className="titlebar-drag" />
      )}
      <div className="titlebar-window-space">
        {import.meta.env.VITE_NATIVE_HOST === 'true' && <NativeWindowControls />}
      </div>
    </div>
  )
}
