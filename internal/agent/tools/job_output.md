Get stdout/stderr from a background shell by ID. Set wait=true to wait for completion, bounded by a timeout: a job still running when the wait ends returns its current output with status running.

The system automatically delivers a finished job's result to you and resumes the work — you do not need to poll this tool in a loop. Start a background job, continue other work or end your turn, and you will be woken with the outcome (status and exit code) when the job completes. Use job_output only for a one-time progress check or to re-read the output of an already-finished job.
