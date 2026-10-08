# Running a web server or API

Apply this reference to the current run task. User, project and mode instructions take priority. Use the project launch command and supported Prime job tools.

The distinguishing concern for servers is lifecycle: launch the owned server, establish readiness, exercise the changed route, and cleanly stop the owned job. Launching alone does not prove feature behavior.

## Structure to document

1. Prerequisites and verified setup for the actual operating system.
2. The project launch command, working directory, environment and port.
3. A readiness check with a bounded deadline.
4. Representative requests and their expected status, headers and body.
5. Shutdown of the specific job this task created.

If setup and verification require a script, keep it in the project or an authorized run skill, with a meaningful exit status. Do not install an illustrative client merely to follow this recipe.

## Background launch

Prime runs each bash call in an independent shell. Shell variables do not persist between calls. Start the actual command through the tool's background option, rather than shell process-detachment syntax:

```json
{"command":"npm run dev","description":"Start API dev server","working_dir":"the-verified-project-path","run_in_background": true}
```

Replace the command and directory with verified project values. Capture the returned shell ID in task context. Keep its identity across subsequent tool calls; never guess a process ID or select a listener merely by port.

Inspect startup evidence using the returned ID:

```json
{"shell_id":"the-returned-shell-id","wait":false}
```

Send that object to job_output. A running state only establishes the process is still running. Read logs and check readiness before exercising the API. Long-lived servers should not use job_output(wait=true) as a readiness signal.

## Readiness and HTTP evidence

For a content-only health endpoint, fetch accepts a fully specified HTTP URL, including a local development URL when the current permissions allow it. Read the actual success response; a denied request does not establish readiness. For status, headers, request methods or richer assertions, use an HTTP client already available in the project, such as its Node runtime or Python standard library. Follow the active permission rules; do not bypass a rejected tool request by switching clients.

For a Node project, the installed runtime can perform one bounded request and print response evidence:

```bash
node -e 'const http=require("node:http");const request=http.get("http://localhost:3000/health",response=>{let body="";response.on("data",chunk=>body+=chunk);response.on("end",()=>console.log(JSON.stringify({status:response.statusCode,headers:response.headers,body})));});request.setTimeout(3000,()=>request.destroy(new Error("request timeout")));request.on("error",error=>{console.error(error.message);process.exitCode=1;});'
```

The host and port must match the owned server. Bound each request and the overall readiness deadline. If connection is refused, inspect startup logs and retry only until the deadline; avoid arbitrary sleep commands or tight tool polling. If no health endpoint exists, check a representative non-mutating route or the documented ready log marker. If readiness never arrives, report the failure and stop the owned job.

## Drive the changed route

After readiness, issue inputs reaching the changed branch. Capture status, headers and body; compare each to expected behavior. Use isolated data, a test service or a safe mode for state-changing requests. Verify a normal path as well as the changed or error path. A successful health endpoint alone does not verify a feature.

## Shutdown

Call job_kill only for the shell ID returned by this task's launch:

```json
{"shell_id":"the-returned-shell-id"}
```

Observe the termination result. Do not stop other services, reuse an unfamiliar job ID, or terminate a process because it occupies the expected port. If the job cannot stop cleanly, report its ID and observed state.

## Details worth recording

Document the port and override, readiness criteria, required versus optional environment variables without secret values, development versus production differences, and dependent services. Keep actual launch, request and stop commands reproducible in the current environment. Store logs and screenshots only at explicitly verified task-owned paths.
