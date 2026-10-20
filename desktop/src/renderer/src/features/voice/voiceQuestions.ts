type Choice = { id: string; label: string; description?: string }
export type VoiceQuestion = {
  id: string
  question: string
  description?: string
  type?: string
  choices?: Choice[]
}
export type VoiceAnswer = {
  request_id: string
  selected_ids?: string[]
  yes?: boolean
  fill_in_text?: string
}

type Translate = (text: string, values?: Record<string, string | number>) => string

const ordinals: string[][] = [
  ['1', 'один', 'первый', 'первая', 'первое', 'first', 'one'],
  ['2', 'два', 'второй', 'вторая', 'второе', 'second', 'two'],
  ['3', 'три', 'третий', 'третья', 'третье', 'third', 'three'],
  ['4', 'четыре', 'четвёртый', 'четвертый', 'четвёртая', 'четвертая', 'fourth', 'four'],
  ['5', 'пять', 'пятый', 'пятая', 'fifth', 'five'],
  ['6', 'шесть', 'шестой', 'шестая', 'sixth', 'six'],
  ['7', 'семь', 'седьмой', 'седьмая', 'seventh', 'seven'],
  ['8', 'восемь', 'восьмой', 'восьмая', 'eighth', 'eight']
]

const yes = /^(да|ага|угу|конечно|верно|согласен|согласна|давай|можно|yes|yeah|yep|sure|ok|окей|хорошо)\b/
const no = /^(нет|не надо|не нужно|неа|отказ|no|nope)\b/
const cancel = /^(отмена|отмени|отменить|cancel|стоп вопрос)\b/

const normalize = (text: string) =>
  text
    .toLowerCase()
    .replace(/ё/g, 'е')
    .replace(/[.,!?;:«»"'()]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()

export function isCancelAnswer(text: string) {
  return cancel.test(normalize(text))
}

export function questionSpeech(question: VoiceQuestion, index: number, total: number, t: Translate) {
  const parts = [total > 1 ? t('Вопрос {value0} из {value1}.', { value0: index + 1, value1: total }) : '']
  parts.push(question.question)
  if (question.description) parts.push(question.description)
  if (question.choices?.length) {
    parts.push(t('Варианты:'))
    question.choices.forEach((choice, position) => parts.push(`${position + 1}. ${choice.label}.`))
    parts.push(
      question.type?.includes('multi')
        ? t('Назовите один или несколько вариантов.')
        : t('Назовите номер или вариант.')
    )
  } else if (question.type === 'yes_no') parts.push(t('Ответьте да или нет.'))
  return parts.filter(Boolean).join(' ')
}

function matchChoices(question: VoiceQuestion, text: string) {
  const spoken = normalize(text)
  const words = new Set(spoken.split(' '))
  const matched: string[] = []
  question.choices?.forEach((choice, index) => {
    const label = normalize(choice.label)
    const byNumber = ordinals[index]?.some((word) => words.has(word))
    const byLabel = label.length > 1 && (spoken.includes(label) || label.split(' ').filter((word) => word.length > 3).some((word) => words.has(word)))
    if (byNumber || byLabel) matched.push(choice.id)
  })
  return question.type?.includes('multi') ? matched : matched.slice(0, 1)
}

export function parseAnswer(question: VoiceQuestion, text: string): VoiceAnswer {
  const spoken = normalize(text)
  const answer: VoiceAnswer = { request_id: question.id }
  if (question.type === 'yes_no') {
    if (yes.test(spoken)) return { ...answer, yes: true }
    if (no.test(spoken)) return { ...answer, yes: false }
    return { ...answer, fill_in_text: text }
  }
  if (question.choices?.length) {
    const selected = matchChoices(question, text)
    if (selected.length) return { ...answer, selected_ids: selected }
  }
  return { ...answer, fill_in_text: text }
}
