package proto

type PluginInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type PluginTask struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Phase string `json:"phase"`
	Text  string `json:"text"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Error string `json:"error,omitempty"`
}

type VoiceUpload struct {
	AudioBase64 string `json:"audio_base64"`
	MIMEType    string `json:"mime_type"`
}

type VoiceTranscript struct {
	Text string `json:"text"`
}
