import { createServer } from 'node:http'
export async function startProvider() {
  const prompts: any[] = []
  const server = createServer((request, response) => {
    if (request.url?.endsWith('/models')) {
      response.setHeader('Content-Type', 'application/json')
      response.end(
        JSON.stringify({
          object: 'list',
          data: [{ id: 'desktop-fixture', object: 'model', owned_by: 'fixture' }]
        })
      )
      return
    }
    const chunks: Buffer[] = []
    request.on('data', (chunk) => chunks.push(chunk))
    request.on('end', () => {
      let body: any
      try {
        body = JSON.parse(Buffer.concat(chunks).toString())
      } catch {
        response.writeHead(400).end()
        return
      }
      prompts.push(body)
      const content = 'Соединение работает. Prime Desktop получил ответ через встроенный CLI.'
      if (!body.stream) {
        response.setHeader('Content-Type', 'application/json')
        response.end(
          JSON.stringify({
            id: 'fixture-completion',
            object: 'chat.completion',
            model: 'desktop-fixture',
            choices: [{ index: 0, message: { role: 'assistant', content }, finish_reason: 'stop' }],
            usage: { prompt_tokens: 12, completion_tokens: 16, total_tokens: 28 }
          })
        )
        return
      }
      response.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' })
      const lastUserIndex = (body.messages || []).findLastIndex(
        (message: any) => message.role === 'user'
      )
      const lastUser = body.messages?.[lastUserIndex]
      const userText =
        typeof lastUser?.content === 'string' ? lastUser.content : JSON.stringify(lastUser?.content)
      if (userText?.includes('HOLD_FIXTURE')) {
        response.write(
          `data: ${JSON.stringify({ id: 'fixture-hold', object: 'chat.completion.chunk', model: 'desktop-fixture', choices: [{ index: 0, delta: { reasoning_content: 'Waiting for the next instruction. '.repeat(20) }, finish_reason: null }] })}\n\n`
        )
        return
      }
      if (
        userText?.includes('QUESTION_FIXTURE') &&
        !body.messages.slice(lastUserIndex + 1).some((message: any) => message.role === 'tool')
      ) {
        response.write(
          `data: ${JSON.stringify({
            id: 'fixture-question',
            object: 'chat.completion.chunk',
            model: 'desktop-fixture',
            choices: [
              {
                index: 0,
                delta: {
                  tool_calls: [
                    {
                      index: 0,
                      id: 'fixture-question-call',
                      type: 'function',
                      function: {
                        name: 'question',
                        arguments: JSON.stringify({
                          questions: [
                            {
                              type: 'single_choice',
                              question: 'Выберите вариант проверки',
                              description: 'Проверка формы вопроса',
                              choices: [
                                { id: 'first', label: 'Первый вариант' },
                                { id: 'second', label: 'Второй вариант' }
                              ]
                            }
                          ]
                        })
                      }
                    }
                  ]
                },
                finish_reason: null
              }
            ]
          })}\n\n`
        )
        response.write(
          `data: ${JSON.stringify({ id: 'fixture-question', object: 'chat.completion.chunk', model: 'desktop-fixture', choices: [{ index: 0, delta: {}, finish_reason: 'tool_calls' }] })}\n\n`
        )
        response.end('data: [DONE]\n\n')
        return
      }
      if (
        userText?.includes('SSH_TOOL_FIXTURE') &&
        !body.messages.slice(lastUserIndex + 1).some((message: any) => message.role === 'tool')
      ) {
        const tool = body.tools?.find((item: any) =>
          item.function?.name?.endsWith('ssh_write_file')
        )
        response.write(
          `data: ${JSON.stringify({ id: 'fixture-ssh-tool', object: 'chat.completion.chunk', model: 'desktop-fixture', choices: [{ index: 0, delta: { tool_calls: [{ index: 0, id: 'fixture-ssh-write', type: 'function', function: { name: tool?.function.name || 'missing_ssh_write_file', arguments: JSON.stringify({ path: 'created-by-tool.txt', content: 'Tool execution verified' }) } }] }, finish_reason: null }] })}\n\n`
        )
        response.write(
          `data: ${JSON.stringify({ id: 'fixture-ssh-tool', object: 'chat.completion.chunk', model: 'desktop-fixture', choices: [{ index: 0, delta: {}, finish_reason: 'tool_calls' }] })}\n\n`
        )
        response.end('data: [DONE]\n\n')
        return
      }
      if (
        userText?.includes('TOOL_FIXTURE') &&
        !body.messages.slice(lastUserIndex + 1).some((message: any) => message.role === 'tool')
      ) {
        response.write(
          `data: ${JSON.stringify({ id: 'fixture-tool', object: 'chat.completion.chunk', model: 'desktop-fixture', choices: [{ index: 0, delta: { tool_calls: [{ index: 0, id: 'fixture-write', type: 'function', function: { name: 'write', arguments: JSON.stringify({ file_path: 'created-by-tool.txt', content: 'Tool execution verified' }) } }] }, finish_reason: null }] })}\n\n`
        )
        response.write(
          `data: ${JSON.stringify({ id: 'fixture-tool', object: 'chat.completion.chunk', model: 'desktop-fixture', choices: [{ index: 0, delta: {}, finish_reason: 'tool_calls' }] })}\n\n`
        )
        response.end('data: [DONE]\n\n')
        return
      }
      const generatingTitle = userText?.startsWith(
        'Generate a concise title for the following content:'
      )
      for (const text of generatingTitle
        ? ['Проверка названия чата']
        : ['Соединение работает. ', 'Prime Desktop получил ответ ', 'через встроенный CLI.'])
        response.write(
          `data: ${JSON.stringify({ id: 'fixture-stream', object: 'chat.completion.chunk', model: 'desktop-fixture', choices: [{ index: 0, delta: { content: text }, finish_reason: null }] })}\n\n`
        )
      response.write(
        `data: ${JSON.stringify({ id: 'fixture-stream', object: 'chat.completion.chunk', model: 'desktop-fixture', choices: [{ index: 0, delta: {}, finish_reason: 'stop' }], usage: { prompt_tokens: 12, completion_tokens: 16, total_tokens: 28 } })}\n\n`
      )
      response.end('data: [DONE]\n\n')
    })
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const address = server.address() as { port: number }
  return {
    url: `http://127.0.0.1:${address.port}/v1`,
    prompts,
    close: async () => {
      server.closeAllConnections()
      await new Promise<void>((resolve) => server.close(() => resolve()))
    }
  }
}
