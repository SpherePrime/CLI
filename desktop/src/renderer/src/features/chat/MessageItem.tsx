import { Fragment, memo, useEffect, useRef, useState } from 'react'
import {
  AlertCircle,
  Archive,
  ArrowUpRight,
  Check,
  CheckCircle2,
  Compass,
  Copy,
  Cpu,
  Sparkles
} from 'lucide-react'
import { MessageAttachment } from './MessageAttachment'
import { MarkdownText } from './MarkdownText'
import { PrimeLogo } from '../../components/PrimeLogo'
import { ToolCard } from './ToolCard'
import { ToolExecutionGroup } from './ToolExecutionGroup'
import { partitionMessageParts } from './toolGroupMerge'
import { AgentCard } from './AgentCard'
import { collectToolImages, ToolGallery, toolImageCaptions, type ToolImage } from './ToolGallery'
import { ToolVideos } from './video/ToolVideos'
import { VideoPlayer } from './video/VideoPlayer'
import { WorkspaceVideo } from './video/WorkspaceVideo'
import { fileNameFromPath, isVideoUrl } from './video/videoMedia'
import { ReasoningBlock } from './ReasoningBlock'
import { useI18n } from '../../i18n'
import type { Message, OrchestratorPhase } from '../../../../shared/contracts'
import { orchestratorPhaseLabel } from '../models/orchestrator'
import { textContent, visibleMessageText } from '../../../../shared/messages'
import {
  canImplementMessagePhoto,
  extractBinaryImage,
  parseDataImageUrl,
  type PreferredDesignImage
} from '../designer/designHandoff'
import '../designer/designer.css'

export const MessageItem = memo(function MessageItem({
  message,
  continued,
  fresh,
  live,
  matched,
  previousOrchestratorPhase,
  onImplementDesign
}: {
  message: Message
  continued: boolean
  fresh: boolean
  live: boolean
  matched: boolean
  previousOrchestratorPhase?: OrchestratorPhase
  onImplementDesign?: (message: Message, preferredImage?: PreferredDesignImage) => void
}) {
  const { t, locale } = useI18n()
  const [copied, setCopied] = useState(false)
  const copiedTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(copiedTimer.current), [])
  const text = textContent(message)
  const parts = message.parts || []
  const messageFinished = parts.some((part) => part.type === 'finish')
  const phase = message.role === 'assistant' ? message.orchestrator_phase : undefined
  const showOrchestratorPhase = Boolean(phase && phase !== previousOrchestratorPhase)
  const canImplementPhoto = canImplementMessagePhoto(
    message.role,
    live,
    Boolean(onImplementDesign)
  )
  if (message.is_summary_message) {
    const finish = parts.find((part) => part.type === 'finish')
    const isError = finish?.data?.reason === 'error'
    return (
      <article
        className={`message assistant summary-message ${fresh ? 'fresh' : ''} ${matched ? 'search-match' : ''}`}
      >
        <div className="message-body">
          {!finish ? (
            <div className="compaction-progress" role="status">
              <span className="working-orb" aria-hidden="true">
                <span />
                <span />
                <span />
              </span>
              <span>{t('Автоматическое сжатие контекста, это может занять несколько минут…')}</span>
            </div>
          ) : isError ? (
            <div className="compaction-status is-error" role="status">
              <AlertCircle size={13} aria-hidden="true" />
              <span>{finish.data?.message || t('Не удалось сжать контекст')}</span>
            </div>
          ) : (
            <div className="compaction-status is-done" role="status">
              <span className="compaction-status-pill">
                <Archive size={13} aria-hidden="true" />
                <span>{t('Контекст сжат')}</span>
              </span>
            </div>
          )}
        </div>
      </article>
    )
  }
  const hasVisibleContent =
    Boolean(visibleMessageText(text, message.role).trim()) ||
    parts.some(
      (part) =>
        (part.type === 'reasoning' && Boolean(part.data?.thinking)) ||
        ['tool_call', 'tool_result', 'shell_command', 'image_url', 'binary'].includes(part.type) ||
        (part.type === 'finish' && ['error', 'canceled'].includes(part.data?.reason))
    )
  const showEmptyAssistantNote =
    message.role === 'assistant' && messageFinished && !hasVisibleContent
  return (
    <article
      className={`message ${message.role} ${continued ? 'continued' : ''} ${fresh ? 'fresh' : ''} ${matched ? 'search-match' : ''}`}
    >
      {!continued && (
        <div className="message-author">
          {message.role === 'user' ? (
            <span className="user-avatar">{t('Вы').slice(0, 1)}</span>
          ) : (
            <span className="prime-avatar">
              <PrimeLogo />
            </span>
          )}
          <strong>
            {message.role === 'user'
              ? t('Вы')
              : message.role === 'tool'
                ? t('Инструмент')
                : 'Prime'}
          </strong>
          {message.created_at && (
            <time>
              {new Date(message.created_at * 1000).toLocaleTimeString(locale, {
                hour: '2-digit',
                minute: '2-digit'
              })}
            </time>
          )}
        </div>
      )}
      <div className="message-body">
        {showOrchestratorPhase && phase && (
          <div className={`orchestrator-phase-chip phase-${phase}`}>
            {phase === 'plan' ? (
              <Compass size={12} aria-hidden="true" />
            ) : phase === 'execute' ? (
              <Cpu size={12} aria-hidden="true" />
            ) : (
              <CheckCircle2 size={12} aria-hidden="true" />
            )}
            <span>
              {orchestratorPhaseLabel(phase, t)}
              {message.model ? ` · ${message.model}` : ''}
            </span>
          </div>
        )}
        {partitionMessageParts(message.parts || []).map((segment) => {
          if (segment.kind === 'tool-group') {
            const groupImages: ToolImage[] = []
            const groupCaptions: string[] = []
            for (const groupPart of segment.parts) {
              const result =
                groupPart.type === 'tool_result' ? groupPart.data : (groupPart.data.result as unknown)
              const partImages = collectToolImages(result)
              const partCaptions = toolImageCaptions(result)
              partImages.forEach((image, idx) => {
                groupImages.push(image)
                groupCaptions.push(partCaptions[idx] || image.path || '')
              })
            }
            return (
              <div className="tool-part" key="turn-tool-group">
                <ToolExecutionGroup
                  parts={segment.parts}
                  messageId={message.id}
                  sessionId={message.session_id}
                  messageFinished={messageFinished}
                />
                <ToolGallery
                  images={groupImages}
                  captions={groupCaptions}
                  implementLabel={t('Сделать этот дизайн')}
                  onImplementImage={
                    canImplementPhoto && onImplementDesign
                      ? (image) =>
                          onImplementDesign(message, {
                            mimeType: image.mime_type,
                            base64: image.data,
                            name: image.path
                          })
                      : undefined
                  }
                />
                {segment.parts.map((groupPart, offset) => {
                  const result =
                    groupPart.type === 'tool_result' ? groupPart.data : (groupPart.data.result as unknown)
                  return <ToolVideos key={offset} result={result} />
                })}
              </div>
            )
          }
          const { part, index } = segment
          if (part.type === 'text' && !part.data.hidden)
            return (
              <div className="markdown" key={index}>
                <MarkdownText text={visibleMessageText(part.data.text || '', message.role)} />
              </div>
            )
          if (part.type === 'reasoning' && part.data.thinking)
            return (
              <ReasoningBlock
                key="turn-reasoning"
                thinking={part.data.thinking}
                startedAt={part.data.started_at}
                finishedAt={part.data.finished_at}
                live={live && !part.data.finished_at}
              />
            )
          if (['tool_call', 'tool_result', 'shell_command'].includes(part.type)) {
            const result = part.type === 'tool_result' ? part.data : (part.data.result as unknown)
            const images = collectToolImages(result)
            return (
              <div className="tool-part" key={index}>
                {part.type === 'tool_call' && part.data?.name === 'agent' ? (
                  <AgentCard
                    part={part}
                    messageId={message.id}
                    sessionId={message.session_id}
                    messageFinished={messageFinished}
                  />
                ) : (
                  <ToolCard part={part} messageFinished={messageFinished} />
                )}
                <ToolGallery
                  images={images}
                  captions={toolImageCaptions(result)}
                  implementLabel={t('Сделать этот дизайн')}
                  onImplementImage={
                    canImplementPhoto && onImplementDesign
                      ? (image) =>
                          onImplementDesign(message, {
                            mimeType: image.mime_type,
                            base64: image.data,
                            name: image.path
                          })
                      : undefined
                  }
                />
                <ToolVideos result={result} />
              </div>
            )
          }
          if (part.type === 'finish' && ['error', 'canceled'].includes(part.data.reason))
            return (
              <div key={index} className="finish-note">
                {part.data.reason === 'canceled'
                  ? t('Задача остановлена')
                  : part.data.message || t('Ошибка выполнения')}
              </div>
            )
          if (part.type === 'image_url') {
            const url = part.data?.url
            if (typeof url === 'string' && /^data:video\//i.test(url)) {
              return <VideoPlayer key={index} src={url} title={t('Видео')} variant="chat" />
            }
            if (typeof url === 'string' && isVideoUrl(url)) {
              return (
                <WorkspaceVideo
                  key={index}
                  src={url}
                  title={fileNameFromPath(url) || t('Видео')}
                  variant="chat"
                />
              )
            }
            const isDataUrl = typeof url === 'string' && url.startsWith('data:')
            const preferred = isDataUrl ? parseDataImageUrl(url) : null
            const attachment = (
              <div className="attachment-note">
                {t('Изображение:')}{' '}
                {isDataUrl ? <img src={url} alt={t('Вложенное изображение')} /> : url}
              </div>
            )
            if (!canImplementPhoto || !onImplementDesign || !preferred) {
              return <Fragment key={index}>{attachment}</Fragment>
            }
            return (
              <div key={index} className="photo-attachment-block">
                {attachment}
                <div className="photo-design-actions">
                  <button
                    type="button"
                    className="design-handoff-trigger photo-design-trigger"
                    onClick={() => onImplementDesign(message, preferred)}
                  >
                    <Sparkles size={13} aria-hidden="true" />
                    <span>{t('Сделать этот дизайн')}</span>
                    <ArrowUpRight size={13} aria-hidden="true" />
                  </button>
                </div>
              </div>
            )
          }
          if (part.type === 'binary') {
            const preferred = extractBinaryImage(part.data)
            if (!canImplementPhoto || !onImplementDesign || !preferred) {
              return <MessageAttachment key={index} data={part.data} />
            }
            return (
              <div key={index} className="photo-attachment-block">
                <MessageAttachment data={part.data} />
                <div className="photo-design-actions">
                  <button
                    type="button"
                    className="design-handoff-trigger photo-design-trigger"
                    onClick={() => onImplementDesign(message, preferred)}
                  >
                    <Sparkles size={13} aria-hidden="true" />
                    <span>{t('Сделать этот дизайн')}</span>
                    <ArrowUpRight size={13} aria-hidden="true" />
                  </button>
                </div>
              </div>
            )
          }
          return null
        })}
        {showEmptyAssistantNote && (
          <div className="finish-note">{t('Модель завершила ответ без текста')}</div>
        )}
      </div>
      {text && message.role === 'user' && (
        <button
          className="copy-message icon"
          aria-label={t('Скопировать сообщение')}
          onClick={() => {
            void navigator.clipboard
              ?.writeText(text)
              .then(() => {
                setCopied(true)
                clearTimeout(copiedTimer.current)
                copiedTimer.current = setTimeout(() => setCopied(false), 1500)
              })
              .catch(() => {})
          }}
        >
          {copied ? <Check size={14} /> : <Copy size={14} />}
        </button>
      )}
    </article>
  )
})
