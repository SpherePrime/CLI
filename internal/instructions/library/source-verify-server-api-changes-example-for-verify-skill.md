# Verifying a server or API change

Apply this reference to the changed route and its claims. Use actual response evidence and supported Prime job lifecycle tools.

## Pattern

1. Launch the project server with bash and "run_in_background": true, recording the returned shell ID.
2. Establish readiness within a bounded deadline.
3. Make representative HTTP requests using an available permitted client.
4. Capture status, headers and body and compare to expected behavior.
5. Stop only the owned job.

## Lifecycle

Read source-run-web-server-api-example for the detailed launch and HTTP-client recipe. Use the actual project start command and verified working directory:

```json
{"command":"npm run dev","description":"Start server verification","working_dir":"the-verified-project-path","run_in_background": true}
```

Each bash call has independent shell state. Preserve the returned ID in task context, then inspect job_output with:

```json
{"shell_id":"the-returned-shell-id","wait":false}
```

Job output is startup evidence; it is not proof the tested endpoint is ready. Use an allowed content check with fetch, or the project's existing Node/Python HTTP client for response status and headers. Inspect permissions and do not work around a rejected request. Bound both individual requests and the total readiness deadline. If readiness fails, inspect logs and report the failure rather than treating a live process as success.

## Worked example

A change adds a Retry-After header to HTTP 429 responses. The claim is that clients can back off correctly. Infer the expected behavior before running checks: the limited request should return 429 with a positive integer Retry-After value, while a normal allowed request remains successful.

Start the owned test server, then use its existing HTTP client to send a bounded sequence of requests to the changed rate-limited route. Choose the count and timing from actual rate-limit configuration instead of blindly assuming ten requests or a particular port. Capture each status and the eventual 429 response headers and body. Check that Retry-After exists and parses to a positive integer. Preserve the normal request response as regression evidence.

A missing header means the change is absent or the test reached the wrong branch. NaN, undefined, negative or malformed values mean the calculation is incorrect. If every request succeeds, the test did not establish the rate-limited branch; inspect configuration and tighten the safe reproduction. Do not report a passing rate-limit test from unrelated successful responses.

## Cleanup and report

Call job_kill with the returned owned ID:

```json
{"shell_id":"the-returned-shell-id"}
```

Observe its result and report exact status/header evidence, failures, skipped paths and pending jobs. Do not terminate unrelated listeners or infer successful shutdown from an intended command. For mutating APIs use isolated data or a documented safe mode and preserve other work.
