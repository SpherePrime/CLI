package voice

import "context"

type SttStatus struct {
	Engine    string
	Model     string
	Language  string
	Languages []string
	Installed bool
}

func Status(dataDir, language string) SttStatus {
	_, installed := ResolveExistingAsrAssets(dataDir)
	return SttStatus{
		Engine:    EngineTinyASR,
		Model:     "en-ru-uk",
		Language:  DisplayLanguage(language),
		Languages: append([]string(nil), AllowedLanguages...),
		Installed: installed,
	}
}

func EnsureInstalled(ctx context.Context, settings Settings, emit func(SetupProgress)) error {
	_, err := EnsureAsrAssets(ctx, settings.DataDir, emit)
	return err
}
