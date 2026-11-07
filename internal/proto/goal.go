package proto

type ChatGoal struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	Status    string `json:"status"`
	Summary   string `json:"summary,omitempty"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type ChatGoalRequest struct {
	Text string `json:"text"`
}
