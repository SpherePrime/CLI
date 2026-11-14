package account

type State struct {
	SignedIn         bool   `json:"signedIn"`
	Waiting          bool   `json:"waiting"`
	Checking         bool   `json:"checking"`
	Email            string `json:"email"`
	LoginURL         string `json:"loginUrl"`
	Error            string `json:"error"`
	NetworkError     bool   `json:"networkError"`
	HasStoredSession bool   `json:"hasStoredSession"`
}
