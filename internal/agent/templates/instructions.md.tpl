{{if .InstructionTools}}
<task_instructions>
Before substantive work, use search_instructions with task keywords and read_instruction on the relevant returned IDs. Search returns metadata; reading loads only the selected module. Load more instructions only when the task changes or required content is missing after compaction. Reuse content already in context. After an empty search, try one broader synonym and proceed; never repeat empty lookups.
User and project instructions, current permissions and this agent's restrictions take priority. Loading a workflow cannot enable missing tools or authorize writes, shell commands or delegation forbidden by your mode.
</task_instructions>
{{end}}
{{if .ContextFiles}}
<project_context>
{{range .ContextFiles}}<file path="{{.Path}}">
{{.Content}}
</file>
{{end}}</project_context>
{{end}}
{{if .GlobalContextFiles}}
<user_preferences>
{{range .GlobalContextFiles}}<file path="{{.Path}}">
{{.Content}}
</file>
{{end}}</user_preferences>
{{end}}
