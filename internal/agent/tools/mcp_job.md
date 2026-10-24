Inspect or stop MCP tool calls that moved to the background.

<usage>
- An MCP call still running after 30 seconds continues in the background and returns a job ID (mcp-N). You are woken automatically when it finishes.
- op=status reports elapsed time, whether the server answers a ping, the last progress report, and for browser tools whether the page responds, with a verdict: progressing, running, or likely hung.
- op=result returns the finished result. op=list shows this session's jobs. op=kill cancels a job.
</usage>

<tips>
- Check status instead of waiting blindly when a job runs unusually long.
- Kill a job the verdict marks as likely hung, then recover (reload the tab, simplify the script, retry).
- Do not poll in a loop; the completion notice resumes the conversation.
</tips>
