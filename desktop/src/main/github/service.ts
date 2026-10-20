import { z } from 'zod'
import type { CommandRunner } from './runner'
import type { GithubRequest, GithubStatus } from '../../shared/github'
import { gitHistory, restoreGitCommit, showGitCommit } from './history'

const issueFields = 'number,title,state,url,updatedAt,author,labels'
const pullFields = `${issueFields},headRefName,baseRefName,isDraft,reviewDecision`
const branchName = z
  .string()
  .min(1)
  .max(255)
  .regex(/^(?!-)[^\s~^:?*[\\]+$/)

const requestSchema = z.discriminatedUnion('action', [
  z.object({ action: z.literal('history') }),
  z.object({
    action: z.literal('showCommit'),
    sha: z.string().regex(/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/i)
  }),
  z.object({
    action: z.literal('restoreCommit'),
    sha: z.string().regex(/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/i),
    expectedHead: z.string().regex(/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/i)
  }),
  z.object({ action: z.enum(['status', 'issues', 'pulls', 'branches']) }),
  z.object({
    action: z.enum(['issue', 'pull', 'diff', 'failedLogs']),
    number: z.number().int().positive()
  }),
  z.object({
    action: z.literal('createPull'),
    head: branchName,
    base: branchName,
    title: z.string().trim().min(1).max(256),
    body: z.string().max(60000),
    draft: z.boolean()
  }),
  z.object({
    action: z.literal('comment'),
    number: z.number().int().positive(),
    body: z.string().trim().min(1).max(60000)
  })
])

const json = async <T>(runner: CommandRunner, args: string[]): Promise<T> =>
  JSON.parse(await runner('gh', args))

async function status(runner: CommandRunner): Promise<GithubStatus> {
  try {
    await runner('gh', ['--version'])
  } catch {
    return {
      available: false,
      authenticated: false,
      error: 'gh не установлен. Установите GitHub CLI и выполните gh auth login.'
    }
  }
  let account: string | undefined
  try {
    account = (await runner('gh', ['api', 'user', '--jq', '.login'])).trim()
  } catch {
    return { available: true, authenticated: false, error: 'Выполните gh auth login в терминале.' }
  }
  const branch =
    (await runner('git', ['rev-parse', '--abbrev-ref', 'HEAD']).catch(() => '')).trim() || undefined
  try {
    const repository = await json<{
      nameWithOwner: string
      url: string
      defaultBranchRef?: { name: string }
    }>(runner, ['repo', 'view', '--json', 'nameWithOwner,url,defaultBranchRef'])
    return {
      available: true,
      authenticated: true,
      account,
      repository: repository.nameWithOwner,
      url: repository.url,
      defaultBranch: repository.defaultBranchRef?.name,
      branch
    }
  } catch {
    return {
      available: true,
      authenticated: true,
      account,
      branch,
      error: 'Проект не связан с репозиторием GitHub.'
    }
  }
}

async function failedLogs(runner: CommandRunner, number: number) {
  const checks = await json<{ name: string; bucket: string; link: string }[]>(runner, [
    'pr',
    'checks',
    String(number),
    '--json',
    'name,bucket,link'
  ]).catch(() => [])
  const failed = checks.filter((check) => check.bucket === 'fail')
  const logs: { name: string; log: string }[] = []
  for (const check of failed.slice(0, 5)) {
    const run = /\/actions\/runs\/(\d+)/.exec(check.link)?.[1]
    if (!run) {
      logs.push({ name: check.name, log: check.link })
      continue
    }
    const output = await runner('gh', ['run', 'view', run, '--log-failed']).catch((error) =>
      String(error.message)
    )
    const lines = output.split('\n')
    logs.push({ name: check.name, log: lines.slice(-150).join('\n').slice(-20000) })
  }
  return logs
}

export async function githubOperation(
  runner: CommandRunner,
  input: unknown,
  workspaceKey?: string
) {
  const request = requestSchema.parse(input) as GithubRequest
  switch (request.action) {
    case 'history':
      return gitHistory(runner)
    case 'showCommit':
      return showGitCommit(runner, request.sha)
    case 'restoreCommit':
      return restoreGitCommit(runner, request.sha, request.expectedHead, workspaceKey)
    case 'status':
      return status(runner)
    case 'issues':
      return json(runner, [
        'issue',
        'list',
        '--state',
        'open',
        '--limit',
        '50',
        '--json',
        issueFields
      ])
    case 'pulls':
      return json(runner, ['pr', 'list', '--state', 'open', '--limit', '50', '--json', pullFields])
    case 'branches': {
      const output = await runner('git', [
        'for-each-ref',
        '--format=%(refname:short)',
        'refs/heads'
      ])
      return output
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
    }
    case 'issue':
      return json(runner, [
        'issue',
        'view',
        String(request.number),
        '--json',
        `${issueFields},body,comments`
      ])
    case 'pull': {
      const number = String(request.number)
      const [detail, checks, repository] = await Promise.all([
        json<any>(runner, [
          'pr',
          'view',
          number,
          '--json',
          `${pullFields},body,comments,reviews,files`
        ]),
        json<any[]>(runner, [
          'pr',
          'checks',
          number,
          '--json',
          'name,state,bucket,link,workflow'
        ]).catch(() => []),
        json<{ nameWithOwner: string }>(runner, ['repo', 'view', '--json', 'nameWithOwner'])
      ])
      const reviewComments = await json<any[]>(runner, [
        'api',
        `repos/${repository.nameWithOwner}/pulls/${number}/comments`,
        '--paginate'
      ])
        .then((items) =>
          items.map((item) => ({
            author: { login: item.user?.login },
            body: item.body,
            path: item.path,
            line: item.line ?? item.original_line,
            createdAt: item.created_at
          }))
        )
        .catch(() => [])
      return { ...detail, checks, reviewComments }
    }
    case 'diff':
      return (await runner('gh', ['pr', 'diff', String(request.number)])).slice(0, 400000)
    case 'failedLogs':
      return failedLogs(runner, request.number)
    case 'createPull': {
      await runner('git', ['push', '-u', 'origin', request.head])
      const args = [
        'pr',
        'create',
        '--head',
        request.head,
        '--base',
        request.base,
        '--title',
        request.title,
        '--body',
        request.body
      ]
      if (request.draft) args.push('--draft')
      return { url: (await runner('gh', args)).trim().split('\n').pop() }
    }
    case 'comment':
      await runner('gh', ['pr', 'comment', String(request.number), '--body', request.body])
      return { ok: true }
  }
}
