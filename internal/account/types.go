package account

type State struct {
	SignedIn         bool   `json:"signedIn"`
	Offline          bool   `json:"offline"`
	Waiting          bool   `json:"waiting"`
	Checking         bool   `json:"checking"`
	Email            string `json:"email"`
	LoginURL         string `json:"loginUrl"`
	Error            string `json:"error"`
	NetworkError     bool   `json:"networkError"`
	HasStoredSession bool   `json:"hasStoredSession"`
}

func (s State) CanAccessWorkspace() bool {
	return s.SignedIn || s.Offline
}
