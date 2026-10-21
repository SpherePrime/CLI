import { expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { splitMarkdownBlocks } from '../../src/renderer/src/features/chat/markdownBlocks'

const html = (text: string) =>
  renderToStaticMarkup(<Markdown remarkPlugins={[remarkGfm]}>{text}</Markdown>).replace(
    />\s+</g,
    '><'
  )
const split = (text: string) => splitMarkdownBlocks(text).map(html).join('')

const samples = [
  '# Title\n\nParagraph one.\n\nParagraph two with **bold**.',
  'Intro\n\n```ts\nconst a = 1\n\nconst b = 2\n```\n\nAfter code.',
  '- one\n- two\n\n- three loose\n\nText after list.',
  '1. first\n\n2. second\n\n   continued\n\n3. third',
  '> quote\n\n> another quote\n\nplain',
  '| a | b |\n| - | - |\n| 1 | 2 |\n\nAfter table.',
  'Para\n\n    indented code\n\n    more code\n\nend',
  '~~~\nunclosed fence\n\nstill code',
  '````md\n```\ninner\n```\n\n````\n\ndone',
  'Term with link [ref].\n\n[ref]: https://example.com',
  'Footnote[^1].\n\n[^1]: Note text.',
  'Line\n\n---\n\nAfter rule\n\n* * *\n\nEnd',
  'Header\n===\n\nSub\n---\n\ntext',
  '- a\n\n-\n\n- c',
  '<!--\n\nhidden\n\n-->\n\nshown',
  '- [ ] task\n- [x] done\n\nNext ~~strike~~ www.example.com',
  '\n\nleading blanks\n\ntrailing blanks\n\n\n',
  ''
]

it('renders split markdown the same as the whole text', () => {
  for (const sample of samples) expect(split(sample)).toBe(html(sample))
})

it('splits only at top-level block boundaries', () => {
  expect(splitMarkdownBlocks('a\n\nb\n\n```\nc\n\nd\n```')).toEqual(['a', 'b', '```\nc\n\nd\n```'])
  expect(splitMarkdownBlocks('- a\n\n- b')).toEqual(['- a\n\n- b'])
})

it('keeps earlier blocks stable while the last one grows', () => {
  const first = splitMarkdownBlocks('Intro\n\nSecond part')
  const second = splitMarkdownBlocks('Intro\n\nSecond part grows')
  expect(second[0]).toBe(first[0])
})
