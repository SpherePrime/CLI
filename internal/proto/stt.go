package proto

// SttCatalog is the speech-to-text state of one workspace: the engine
// currently configured and every provider with its downloadable models.
type SttCatalog struct {
	Engine    string        `json:"engine"`
	Model     string        `json:"model"`
	Providers []SttProvider `json:"providers"`
}

// SttProvider is one transcription engine the desktop can offer.
type SttProvider struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Available   bool       `json:"available"`
	Models      []SttModel `json:"models"`
}

// SttModel is one downloadable model of a provider.
type SttModel struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Language    string `json:"language"`
	SizeBytes   int64  `json:"size_bytes"`
	Recommended bool   `json:"recommended"`
	Installed   bool   `json:"installed"`
	Active      bool   `json:"active"`
}

// SttModelRequest names a model to install, remove or select.
type SttModelRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// SttSelection echoes the engine and model after a select.
type SttSelection struct {
	Engine string `json:"engine"`
	Model  string `json:"model"`
}

// Started confirms an asynchronous operation was accepted.
type Started struct {
	Started bool `json:"started"`
}
