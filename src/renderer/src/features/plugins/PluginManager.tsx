import { useEffect, useState } from 'react'
import { Mic, Puzzle, RefreshCw, Check, LoaderCircle } from 'lucide-react'
import { Modal } from '../../components/Modal'
import type { Workspace } from '../../app/useWorkspace'
export function PluginManager({
  workspace: w,
  close
}: {
  workspace: Workspace
  close: () => void
}) {
  const [plugins, setPlugins] = useState<any[]>([])
  const [tasks, setTasks] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [working, setWorking] = useState('')
  const [error, setError] = useState('')
  async function refresh() {
    try {
      const [items, active] = await Promise.all([w.call('plugins'), w.call('pluginTasks')])
      setPlugins(Array.isArray(items) ? items : items.plugins || [])
      setTasks(active || [])
    } catch (error: any) {
      setError(error.message)
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => {
    void refresh()
    const timer = setInterval(() => void refresh(), 2000)
    return () => clearInterval(timer)
  }, [w.project?.id])
  return (
    <Modal title="Плагины Prime" close={close}>
      <p className="muted">
        Дополнительные возможности встроенного CLI. Плагины устанавливаются по вашему выбору.
      </p>
      {!w.project && <p className="empty-small">Сначала откройте проект</p>}
      {error && <p className="error-text">{error}</p>}
      {loading && <p className="muted">Загружаем плагины…</p>}
      {plugins.map((plugin) => (
        <div className="plugin-card" key={plugin.name}>
          <span className="plugin-icon">
            {plugin.name === 'voice' ? <Mic size={22} /> : <Puzzle size={22} />}
          </span>
          <div>
            <h3>{plugin.title || plugin.name}</h3>
            <p>{plugin.description || 'Дополнительная возможность Prime'}</p>
            {plugin.name === 'voice' && <small>Alt+V · запись и распознавание в поле ввода</small>}
          </div>
          <button
            className={plugin.enabled ? 'quiet' : 'primary'}
            disabled={
              working === plugin.name ||
              tasks.some((task) => task.name === plugin.name && task.state === 'running')
            }
            onClick={async () => {
              setWorking(plugin.name)
              setError('')
              try {
                await w.call(plugin.enabled ? 'pluginDisable' : 'pluginEnable', {
                  name: plugin.name
                })
                await refresh()
              } catch (error: any) {
                setError(error.message)
              } finally {
                setWorking('')
              }
            }}
          >
            {working === plugin.name ? (
              <LoaderCircle size={14} className="spin" />
            ) : plugin.enabled ? (
              'Выключить'
            ) : (
              'Включить'
            )}
          </button>
        </div>
      ))}
      {tasks.map((task, index) => (
        <div className="plugin-task" key={index}>
          {task.state === 'running' ? (
            <LoaderCircle size={15} className="spin" />
          ) : (
            <Check size={15} />
          )}
          <span>
            {task.name} · {task.text || task.phase || task.state}
            {task.error && <small className="error-text">{task.error}</small>}
          </span>
          {task.total > 0 && <progress value={task.done} max={task.total} />}
        </div>
      ))}
    </Modal>
  )
}
