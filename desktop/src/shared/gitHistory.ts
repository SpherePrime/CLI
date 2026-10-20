export type GitHistoryCommit = {
  sha: string
  parents: string[]
  title: string
  author: string
  date: string
  refs: string
}

export type GitHistory = {
  commits: GitHistoryCommit[]
  currentHead: string
  branch: string
  clean: boolean
}

export type GitCommitDetail = {
  commit: GitHistoryCommit
  currentHead: string
  diff: string
  truncated: boolean
}

export type GitRestoreResult = {
  previousHead: string
  currentHead: string
  restoredCommit: string
  changed: boolean
}
