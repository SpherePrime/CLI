import type { GithubComment, GithubIssueDetail, GithubPullDetail } from '../../../../shared/github'

const comments = (items: GithubComment[]) =>
  items
    .filter((item) => item.body?.trim())
    .map((item) => `- ${item.author?.login ?? 'unknown'}${item.path ? ` (${item.path}${item.line ? `:${item.line}` : ''})` : ''}: ${item.body.trim()}`)
    .join('\n')

export function issuePrompt(issue: GithubIssueDetail) {
  return [
    `Resolve GitHub issue #${issue.number}: ${issue.title}`,
    issue.url,
    issue.body?.trim() ? `Description:\n${issue.body.trim()}` : '',
    issue.comments?.length ? `Discussion:\n${comments(issue.comments)}` : '',
    'Investigate the code, implement the fix and verify it. Do not push or publish anything to GitHub.'
  ]
    .filter(Boolean)
    .join('\n\n')
}

export function reviewPrompt(pull: GithubPullDetail) {
  const reviews = pull.reviews
    .filter((review) => review.body?.trim())
    .map((review) => `- ${review.author?.login ?? 'unknown'} (${review.state}): ${review.body.trim()}`)
    .join('\n')
  return [
    `Address the review feedback on pull request #${pull.number}: ${pull.title} (branch ${pull.headRefName}).`,
    reviews ? `Reviews:\n${reviews}` : '',
    pull.reviewComments.length ? `Line comments:\n${comments(pull.reviewComments)}` : '',
    pull.comments.length ? `Conversation:\n${comments(pull.comments)}` : '',
    'Make the requested changes on this branch and verify them. Do not push.'
  ]
    .filter(Boolean)
    .join('\n\n')
}

export function checksPrompt(pull: GithubPullDetail, logs: { name: string; log: string }[]) {
  return [
    `Fix the failing checks of pull request #${pull.number}: ${pull.title} (branch ${pull.headRefName}).`,
    ...logs.map((item) => `Check "${item.name}":\n\`\`\`\n${item.log}\n\`\`\``),
    'Find the root cause, fix it and run the relevant checks locally. Do not push.'
  ].join('\n\n')
}
