import type { GithubComment } from '../../../../shared/github'
import { useI18n } from '../../i18n'

export function GithubComments({ comments }: { comments: GithubComment[] }) {
  const { t } = useI18n()
  if (!comments.length) return null
  return (
    <section className="github-comments" aria-label={t('Комментарии')}>
      <h4>
        {t('Комментарии')} <span>{comments.length}</span>
      </h4>
      {comments.map((comment, index) => (
        <article className="github-comment" key={`${comment.createdAt}-${index}`}>
          <header>
            <strong>{comment.author?.login ? `@${comment.author.login}` : 'GitHub'}</strong>
            {comment.createdAt && (
              <time dateTime={comment.createdAt}>
                {new Date(comment.createdAt).toLocaleString()}
              </time>
            )}
          </header>
          {comment.path && (
            <p className="github-meta">
              {comment.path}
              {comment.line ? `:${comment.line}` : ''}
            </p>
          )}
          <pre className="github-body">{comment.body}</pre>
        </article>
      ))}
    </section>
  )
}
