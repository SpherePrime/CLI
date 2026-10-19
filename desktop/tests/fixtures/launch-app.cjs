const http = require('node:http')
const server = http.createServer((request, response) => {
  response.setHeader('Content-Type', 'text/html')
  response.end('<html><body><h1>Fixture app</h1><script>console.error("FIXTURE_BROWSER_ERROR")</script></body></html>')
})
server.listen(0, '127.0.0.1', () => console.log(`Local: http://localhost:${server.address().port}/`))