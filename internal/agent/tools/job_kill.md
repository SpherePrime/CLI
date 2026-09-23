Terminate a background shell process.

<usage>
- Provide the shell ID returned from a background bash execution
- Cancels the running process tree and cleans up tracking
</usage>

<features>
- Stop long-running background processes
- Clean up completed background shells
- Immediately terminates the process
</features>

<tips>
- Use this when you need to stop a background process
- The process tree is terminated; on Windows descendant processes are killed too
- The call returns as soon as cancellation is issued, so the exit lands asynchronously
- After killing, the shell ID becomes invalid
</tips>
