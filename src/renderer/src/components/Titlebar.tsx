import { PrimeLogo } from './PrimeLogo'
export function Titlebar() {
  return (
    <div className="titlebar">
      <div className="titlebar-brand">
        <PrimeLogo />
        <span>
          Prime<span className="brand-word-muted">Desktop</span>
        </span>
      </div>
      <div className="titlebar-drag" />
    </div>
  )
}
