export type GithubUser = { login: string }
export type GithubListState = 'open' | 'closed' | 'all'

export type GithubIssue = {
  number: number
  title: string
  state: string
  url: string
  updatedAt: string
  author?: GithubUser
  labels?: { name: string; color?: string }[]
}

export type GithubPull = GithubIssue & {
  headRefName: string
  baseRefName: string
  isDraft: boolean
  reviewDecision?: string
}

export type GithubComment = {
  author?: GithubUser
  body: string
  createdAt?: string
  path?: string
  line?: number
}

export type GithubCheck = {
  name: string
  state: string
  bucket: string
  link: string
  workflow?: string
}

export type GithubIssueDetail = GithubIssue & { body: string; comments: GithubComment[] }

export type GithubPullDetail = GithubPull & {
  body: string
  comments: GithubComment[]
  reviews: { author?: GithubUser; state: string; body: string }[]
  files: { path: string; additions: number; deletions: number }[]
  checks: GithubCheck[]
  checksError?: string
  reviewComments: GithubComment[]
}

export type GithubStatus = {
  available: boolean
  authenticated: boolean
  account?: string
  repository?: string
  url?: string
  defaultBranch?: string
  branch?: string
  error?: string
}

export type GithubRequest =
  | { action: 'history' }
  | { action: 'showCommit'; sha: string }
  | { action: 'restoreCommit'; sha: string; expectedHead: string }
  | { action: 'status' | 'branches' }
  | { action: 'issues' | 'pulls'; state?: GithubListState; limit?: number }
  | { action: 'issue' | 'pull' | 'diff'; number: number }
  | { action: 'failedLogs'; number: number }
  | {
      action: 'createPull'
      head: string
      base: string
      title: string
      body: string
      draft: boolean
    }
  | { action: 'comment'; number: number; body: string }
