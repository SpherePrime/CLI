package proto

import "time"

type MCPChannelMessage struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	WorkspacePath string    `json:"workspace_path"`
	Content       string    `json:"content"`
	ReceivedAt    time.Time `json:"received_at"`
}
