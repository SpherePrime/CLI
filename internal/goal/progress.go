package goal

import (
	"strings"

	"github.com/SpherePrime/CLI/internal/session"
)

type Progress struct {
	Total     int
	Completed int
	Next      string
}

func ProgressOf(todos []session.Todo) Progress {
	progress := Progress{Total: len(todos)}
	for _, todo := range todos {
		if todo.Status == session.TodoStatusCompleted {
			progress.Completed++
		} else if progress.Next == "" {
			progress.Next = todo.Content
		}
	}
	for _, todo := range todos {
		if todo.Status == session.TodoStatusInProgress {
			progress.Next = todo.Content
			break
		}
	}
	return progress
}

func (p Progress) Finished() bool {
	return p.Total > 0 && p.Completed == p.Total
}

func signature(todos []session.Todo) string {
	var builder strings.Builder
	for _, todo := range todos {
		builder.WriteString(string(todo.Status))
		builder.WriteByte('\x1f')
		builder.WriteString(todo.Content)
		builder.WriteByte('\x1e')
	}
	return builder.String()
}
