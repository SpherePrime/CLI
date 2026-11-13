import { Component, type ReactNode } from 'react'
import { RotateCcw, TriangleAlert } from 'lucide-react'
import { EmptyState } from './EmptyState'
import { useI18n } from '../i18n'

type PanelBoundaryProps = { children: ReactNode; resetKey?: unknown; compact?: boolean }
type PanelBoundaryState = { error: Error | null; resetKey: unknown }

export class PanelBoundary extends Component<PanelBoundaryProps, PanelBoundaryState> {
  state: PanelBoundaryState = { error: null, resetKey: this.props.resetKey }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  static getDerivedStateFromProps(props: PanelBoundaryProps, state: PanelBoundaryState) {
    if (props.resetKey === state.resetKey) return null
    return { error: null, resetKey: props.resetKey }
  }

  componentDidCatch(error: Error) {
    console.error(error)
  }

  render() {
    if (!this.state.error) return this.props.children
    return (
      <PanelFailure
        message={this.state.error.message}
        compact={this.props.compact}
        retry={() => this.setState({ error: null })}
      />
    )
  }
}

function PanelFailure({
  message,
  compact,
  retry
}: {
  message: string
  compact?: boolean
  retry: () => void
}) {
  const { t } = useI18n()
  return (
    <div className="panel-failure" role="alert">
      <EmptyState
        icon={TriangleAlert}
        title={t('Панель не открылась')}
        text={message || t('Неизвестная ошибка')}
        compact={compact ?? true}
        action={{ label: t('Повторить'), icon: RotateCcw, onClick: retry }}
      />
    </div>
  )
}
