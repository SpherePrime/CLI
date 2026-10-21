package proto

type MessagePageTiming struct {
	StartedAt int64            `json:"started_at,omitempty"`
	Completed map[string]int64 `json:"completed"`
}

type MessagePage struct {
	Messages  []Message         `json:"messages"`
	HasMore   bool              `json:"has_more"`
	HasBefore bool              `json:"has_before"`
	HasAfter  bool              `json:"has_after"`
	Before    string            `json:"before,omitempty"`
	After     string            `json:"after,omitempty"`
	Timing    MessagePageTiming `json:"timing"`
}
