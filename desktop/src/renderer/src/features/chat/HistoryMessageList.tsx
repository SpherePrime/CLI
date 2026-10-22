import { useSyncExternalStore, type ComponentProps } from 'react'
import { MessageList } from './MessageList'
import type { MessageHistory } from './useMessageHistory'

type HistoryMessageListProps = { history: MessageHistory } & Omit<
  ComponentProps<typeof MessageList>,
  'messages' | 'hasBefore' | 'hasAfter' | 'historyTiming'
>

export function HistoryMessageList({ history, ...props }: HistoryMessageListProps) {
  const page = useSyncExternalStore(history.subscribe, history.getPage)
  return (
    <MessageList
      {...props}
      messages={page.messages}
      hasBefore={page.has_before}
      hasAfter={page.has_after}
      historyTiming={page.timing}
    />
  )
}
