import { useCallback, useEffect, useRef, useState } from 'react'
import type {
  GithubIssue,
  GithubListState,
  GithubPull,
  GithubRequest,
  GithubStatus
} from '../../../../shared/github'

export type GithubListResult<T> = { items: T[]; error: string }

export function githubErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}

export function useGithubRepository(api: (request: GithubRequest) => Promise<any>) {
  const [status, setStatus] = useState<GithubStatus>()
  const [state, setState] = useState<GithubListState>('all')
  const [limit, setLimit] = useState(50)
  const [issues, setIssues] = useState<GithubListResult<GithubIssue>>({ items: [], error: '' })
  const [pulls, setPulls] = useState<GithubListResult<GithubPull>>({ items: [], error: '' })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const requestSequence = useRef(0)
  const refresh = useCallback(async () => {
    const sequence = ++requestSequence.current
    setLoading(true)
    setError('')
    try {
      const next: GithubStatus = await api({ action: 'status' })
      if (sequence !== requestSequence.current) return
      setStatus(next)
      if (!next.repository) {
        setIssues({ items: [], error: '' })
        setPulls({ items: [], error: '' })
        return
      }
      const results = await Promise.allSettled([
        api({ action: 'issues', state, limit }),
        api({ action: 'pulls', state, limit })
      ])
      if (sequence !== requestSequence.current) return
      const listResult = <T>(result: PromiseSettledResult<T[]>): GithubListResult<T> =>
        result.status === 'fulfilled'
          ? { items: result.value, error: '' }
          : { items: [], error: githubErrorMessage(result.reason) }
      setIssues(listResult<GithubIssue>(results[0]))
      setPulls(listResult<GithubPull>(results[1]))
    } catch (error) {
      if (sequence === requestSequence.current) setError(githubErrorMessage(error))
    } finally {
      if (sequence === requestSequence.current) setLoading(false)
    }
  }, [api, state, limit])
  useEffect(() => {
    void refresh()
    return () => {
      requestSequence.current++
    }
  }, [refresh])
  const changeState = (next: GithubListState) => {
    requestSequence.current++
    setLoading(true)
    setState(next)
    setLimit(50)
  }
  return {
    status,
    state,
    limit,
    issues,
    pulls,
    loading,
    error,
    refresh,
    changeState,
    loadMore: () => setLimit((current) => Math.min(1000, current + 50))
  }
}
