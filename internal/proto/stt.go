package proto

type SttCatalog struct {
	Engine    string   `json:"engine"`
	Model     string   `json:"model"`
	Language  string   `json:"language"`
	Languages []string `json:"languages"`
	Installed bool     `json:"installed"`
}

type SttModelRequest struct {
	Language string `json:"language,omitempty"`
}

type SttSelection struct {
	Engine   string `json:"engine"`
	Model    string `json:"model"`
	Language string `json:"language"`
}
