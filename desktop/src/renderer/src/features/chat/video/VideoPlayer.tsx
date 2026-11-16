import {
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type MouseEvent
} from 'react'
import {
  AlertCircle,
  Film,
  LoaderCircle,
  Maximize2,
  Pause,
  Play,
  RotateCcw,
  Volume2,
  VolumeX
} from 'lucide-react'
import { useI18n } from '../../../i18n'
import { fileNameFromPath, formatVideoTime, videoFormatLabel } from './videoMedia'
import { VideoLightbox } from './VideoLightbox'
import './videoPlayer.css'

export type VideoPlayerVariant = 'chat' | 'lightbox' | 'editor'

const PLAYBACK_RATES = [1, 1.25, 1.5, 2, 0.5]

export interface VideoPlayerProps {
  src: string
  title?: string
  mime?: string
  variant?: VideoPlayerVariant
  autoPlay?: boolean
  loading?: boolean
  error?: boolean
  onExpand?: () => void
  onRetry?: () => void
}

export function VideoPlayer({
  src,
  title,
  mime,
  variant = 'chat',
  autoPlay = false,
  loading: externalLoading = false,
  error: externalError = false,
  onExpand,
  onRetry
}: VideoPlayerProps) {
  const { t } = useI18n()
  const videoRef = useRef<HTMLVideoElement>(null)
  const [playing, setPlaying] = useState(false)
  const [muted, setMuted] = useState(false)
  const [duration, setDuration] = useState(0)
  const [currentTime, setCurrentTime] = useState(0)
  const [playbackRate, setPlaybackRate] = useState(1)
  const [metadataLoaded, setMetadataLoaded] = useState(false)
  const [dimensions, setDimensions] = useState<{ width: number; height: number } | null>(null)
  const [videoError, setVideoError] = useState(false)
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const [retryCount, setRetryCount] = useState(0)

  const hasError = externalError || videoError
  const isLoading = !hasError && (externalLoading || (Boolean(src) && !metadataLoaded))
  const displayTitle = title || fileNameFromPath(src) || t('Видео')
  const formatLabel = videoFormatLabel(mime, title || src)
  const dimensionLabel =
    dimensions && dimensions.width > 0 && dimensions.height > 0
      ? `${dimensions.width}×${dimensions.height}`
      : ''
  const metaPill = metadataLoaded ? [formatLabel, dimensionLabel].filter(Boolean).join(' · ') : ''

  useEffect(() => {
    setPlaying(false)
    setDuration(0)
    setCurrentTime(0)
    setDimensions(null)
    setMetadataLoaded(false)
    setVideoError(false)

    const video = videoRef.current
    if (!video || !src) return
    if (video.readyState >= 1) {
      if (Number.isFinite(video.duration) && video.duration > 0) {
        setDuration(video.duration)
      }
      if (video.videoWidth > 0 && video.videoHeight > 0) {
        setDimensions({ width: video.videoWidth, height: video.videoHeight })
      }
      setMetadataLoaded(true)
    }
  }, [src, retryCount])

  useEffect(() => {
    const video = videoRef.current
    if (video) video.playbackRate = playbackRate
  }, [playbackRate])

  useEffect(() => {
    const video = videoRef.current
    if (video) video.muted = muted
  }, [muted])

  useEffect(() => {
    const video = videoRef.current
    if (!autoPlay || !src || !video) return
    const attempt = video.play()
    if (attempt && typeof attempt.catch === 'function') {
      attempt.catch(() => {})
    }
  }, [autoPlay, src])

  function togglePlay() {
    const video = videoRef.current
    if (!video || !src || isLoading || hasError) return
    if (video.paused || video.ended) {
      const attempt = video.play()
      if (attempt && typeof attempt.catch === 'function') {
        attempt.catch(() => {})
      }
    } else {
      video.pause()
    }
  }

  function toggleMute() {
    setMuted((current) => {
      const next = !current
      if (videoRef.current) videoRef.current.muted = next
      return next
    })
  }

  function cycleSpeed() {
    const index = PLAYBACK_RATES.indexOf(playbackRate)
    const next = PLAYBACK_RATES[(index + 1) % PLAYBACK_RATES.length]
    setPlaybackRate(next)
  }

  function seekTo(nextTime: number) {
    const video = videoRef.current
    const max = duration > 0 ? duration : 0
    const clamped = Math.min(max, Math.max(0, nextTime))
    setCurrentTime(clamped)
    if (video) video.currentTime = clamped
  }

  function handleRetry(event: MouseEvent<HTMLButtonElement>) {
    event.stopPropagation()
    setVideoError(false)
    setMetadataLoaded(false)
    setRetryCount((count) => count + 1)
    onRetry?.()
    if (videoRef.current && src) {
      videoRef.current.load()
    }
  }

  function handleExpand() {
    videoRef.current?.pause()
    if (onExpand) {
      onExpand()
    } else {
      setLightboxOpen(true)
    }
  }

  function handlePlayerKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const target = event.target as HTMLElement | null
    if (target?.tagName === 'BUTTON' || target?.tagName === 'INPUT') return
    if (event.key === ' ' || event.key === 'Enter') {
      event.preventDefault()
      togglePlay()
    } else if (event.key === 'ArrowLeft') {
      event.preventDefault()
      seekTo(currentTime - (duration > 10 ? 5 : 0.5))
    } else if (event.key === 'ArrowRight') {
      event.preventDefault()
      seekTo(currentTime + (duration > 10 ? 5 : 0.5))
    } else if (event.key === 'm' || event.key === 'M') {
      event.preventDefault()
      toggleMute()
    }
  }

  function handleScrubberKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    const step = duration > 10 ? 5 : 0.5
    if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') {
      event.preventDefault()
      seekTo(currentTime - step)
    } else if (event.key === 'ArrowRight' || event.key === 'ArrowUp') {
      event.preventDefault()
      seekTo(currentTime + step)
    } else if (event.key === 'Home') {
      event.preventDefault()
      seekTo(0)
    } else if (event.key === 'End') {
      event.preventDefault()
      seekTo(duration)
    }
  }

  const progress = duration > 0 ? Math.min(100, Math.max(0, (currentTime / duration) * 100)) : 0

  return (
    <>
      <div
        className={`video-player variant-${variant}`}
        tabIndex={0}
        onKeyDown={handlePlayerKeyDown}
        aria-label={displayTitle}
      >
        <div className="video-player-header">
          <div className="video-player-heading">
            <Film size={14} className="video-player-icon" aria-hidden="true" />
            <span className="video-player-title" title={displayTitle}>
              {displayTitle}
            </span>
          </div>
          {metaPill && <span className="video-player-meta">{metaPill}</span>}
        </div>
        <div className="video-stage" onClick={togglePlay}>
          <video
            ref={videoRef}
            src={src || undefined}
            preload="metadata"
            playsInline
            autoPlay={autoPlay}
            muted={muted}
            onLoadedMetadata={(event) => {
              const video = event.currentTarget
              if (Number.isFinite(video.duration) && video.duration > 0) {
                setDuration(video.duration)
              }
              if (video.videoWidth > 0 && video.videoHeight > 0) {
                setDimensions({ width: video.videoWidth, height: video.videoHeight })
              }
              setMetadataLoaded(true)
              setVideoError(false)
            }}
            onDurationChange={(event) => {
              const nextDuration = event.currentTarget.duration
              if (Number.isFinite(nextDuration) && nextDuration > 0) {
                setDuration(nextDuration)
              }
            }}
            onTimeUpdate={(event) => {
              setCurrentTime(event.currentTarget.currentTime)
            }}
            onPlay={() => {
              setPlaying(true)
              setVideoError(false)
            }}
            onPause={() => setPlaying(false)}
            onEnded={() => setPlaying(false)}
            onError={() => {
              setVideoError(true)
              setPlaying(false)
            }}
          />
          {!playing && !isLoading && !hasError && Boolean(src) && (
            <button
              type="button"
              className="video-stage-play"
              aria-label={t('Воспроизвести')}
              title={t('Воспроизвести')}
              onClick={(event) => {
                event.stopPropagation()
                togglePlay()
              }}
            >
              <Play size={22} aria-hidden="true" />
            </button>
          )}
          {isLoading && (
            <div className="video-stage-status" role="status" onClick={(event) => event.stopPropagation()}>
              <LoaderCircle size={20} className="spin" aria-hidden="true" />
              <span>{t('Загрузка…')}</span>
            </div>
          )}
          {hasError && (
            <div className="video-stage-error" role="alert" onClick={(event) => event.stopPropagation()}>
              <AlertCircle size={20} aria-hidden="true" />
              <span>{t('Не удалось загрузить видео')}</span>
              <button type="button" className="video-retry-btn" onClick={handleRetry}>
                <RotateCcw size={13} aria-hidden="true" />
                <span>{t('Повторить')}</span>
              </button>
            </div>
          )}
        </div>
        <div className="video-controls">
          <button
            type="button"
            className="video-ctrl-btn"
            disabled={!src || isLoading || hasError}
            aria-label={playing ? t('Пауза') : t('Воспроизвести')}
            title={playing ? t('Пауза') : t('Воспроизвести')}
            onClick={togglePlay}
          >
            {playing ? <Pause size={15} aria-hidden="true" /> : <Play size={15} aria-hidden="true" />}
          </button>
          <span className="video-time">
            {formatVideoTime(currentTime)} / {formatVideoTime(duration)}
          </span>
          <input
            type="range"
            className="video-scrubber"
            min={0}
            max={duration > 0 ? duration : 0}
            step="any"
            value={Math.min(currentTime, duration || 0)}
            disabled={!duration || hasError}
            aria-label={t('Перемотка видео')}
            style={{ '--range-progress': `${progress}%` } as CSSProperties}
            onChange={(event) => seekTo(Number(event.currentTarget.value))}
            onKeyDown={handleScrubberKeyDown}
          />
          <button
            type="button"
            className="video-ctrl-btn"
            aria-label={muted ? t('Включить звук') : t('Выключить звук')}
            title={muted ? t('Включить звук') : t('Выключить звук')}
            onClick={toggleMute}
          >
            {muted ? <VolumeX size={15} aria-hidden="true" /> : <Volume2 size={15} aria-hidden="true" />}
          </button>
          <button
            type="button"
            className="video-speed-btn"
            aria-label={t('Скорость воспроизведения')}
            title={t('Скорость воспроизведения')}
            onClick={cycleSpeed}
          >
            {playbackRate}×
          </button>
          {variant !== 'lightbox' && (
            <button
              type="button"
              className="video-ctrl-btn"
              disabled={!src || hasError}
              aria-label={t('Открыть видео на весь экран')}
              title={t('Открыть видео на весь экран')}
              onClick={handleExpand}
            >
              <Maximize2 size={15} aria-hidden="true" />
            </button>
          )}
        </div>
      </div>
      {variant !== 'lightbox' && lightboxOpen && (
        <VideoLightbox
          src={src}
          title={displayTitle}
          mime={mime}
          close={() => setLightboxOpen(false)}
        />
      )}
    </>
  )
}
