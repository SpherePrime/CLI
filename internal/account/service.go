package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/pkg/browser"
)

const (
	defaultBaseURL      = "https://vpn.sphereprime.ru"
	defaultTimeout      = 10 * time.Second
	defaultPollInterval = 2 * time.Second
	defaultMaxAttempts  = 150
	accountFileName     = "prime-account.bin"

	errSessionExpired     = "Сессия истекла. Войдите снова."
	errSessionCheckFailed = "Не удалось проверить сессию. Проверьте подключение и повторите попытку."
	errBrowserOpenFailed  = "Не удалось открыть браузер автоматически. Откройте или скопируйте ссылку вручную."
	errLoginTimeout       = "Время ожидания истекло. Повторите вход."
	errLoginNetwork       = "Ошибка сети при проверке входа. Повторите попытку."
)

type Option func(*Service)

func WithStoragePath(path string) Option {
	return func(s *Service) {
		if strings.TrimSpace(path) != "" {
			s.storagePath = path
		}
	}
}

func WithBaseURL(baseURL string) Option {
	return func(s *Service) {
		if trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/"); trimmed != "" {
			s.baseURL = trimmed
		}
	}
}

func WithOpenBrowser(fn func(string) error) Option {
	return func(s *Service) {
		if fn != nil {
			s.openBrowser = fn
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(s *Service) {
		if client != nil {
			s.httpClient = client
		}
	}
}

func WithPollInterval(d time.Duration) Option {
	return func(s *Service) {
		if d > 0 {
			s.pollInterval = d
		}
	}
}

func WithMaxAttempts(n int) Option {
	return func(s *Service) {
		if n > 0 {
			s.maxAttempts = n
		}
	}
}

type Service struct {
	mu           sync.Mutex
	fileMu       sync.Mutex
	restoreMu    sync.Mutex
	storagePath  string
	baseURL      string
	openBrowser  func(string) error
	httpClient   *http.Client
	pollInterval time.Duration
	maxAttempts  int

	state        State
	token        string
	checkedToken string
	flow         uint64
	pollCancel   context.CancelFunc
	wg           sync.WaitGroup
	closed       bool
}

func NewService(opts ...Option) *Service {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PRIME_ACCOUNT_URL")), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	svc := &Service{
		baseURL:      baseURL,
		openBrowser:  browser.OpenURL,
		pollInterval: defaultPollInterval,
		maxAttempts:  defaultMaxAttempts,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	svc.storagePath = svc.DefaultPath()

	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *Service) DefaultPath() string {
	return filepath.Join(config.GlobalDataDir(), accountFileName)
}

func (s *Service) Restore(ctx context.Context) State {
	if ctx == nil {
		ctx = context.Background()
	}

	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()

	s.mu.Lock()
	if s.closed || s.state.Waiting {
		snapshot := s.state
		s.mu.Unlock()
		return snapshot
	}
	flow := s.flow
	storagePath := s.storagePath
	baseURL := s.baseURL
	s.mu.Unlock()

	s.fileMu.Lock()
	token, loadErr := loadToken(storagePath)
	s.fileMu.Unlock()

	if loadErr != nil {
		if !os.IsNotExist(loadErr) {
			s.fileMu.Lock()
			s.mu.Lock()
			if !s.closed && s.flow == flow && !s.state.Waiting {
				_ = deleteToken(storagePath)
				s.token = ""
				s.checkedToken = ""
				s.state = State{SignedIn: false}
			}
			snapshot := s.state
			s.mu.Unlock()
			s.fileMu.Unlock()
			return snapshot
		}
		s.mu.Lock()
		if !s.closed && s.flow == flow && !s.state.Waiting {
			s.token = ""
			s.checkedToken = ""
			s.state = State{SignedIn: false}
		}
		snapshot := s.state
		s.mu.Unlock()
		return snapshot
	}

	if token == "" {
		s.mu.Lock()
		if !s.closed && s.flow == flow && !s.state.Waiting {
			s.token = ""
			s.checkedToken = ""
			s.state = State{SignedIn: false}
		}
		snapshot := s.state
		s.mu.Unlock()
		return snapshot
	}

	s.mu.Lock()
	if s.closed || s.flow != flow || s.state.Waiting {
		snapshot := s.state
		s.mu.Unlock()
		return snapshot
	}
	s.state.Checking = true
	s.state.HasStoredSession = true
	s.mu.Unlock()

	email, statusCode, reqErr := s.fetchMe(ctx, baseURL, token)

	if reqErr != nil || statusCode >= 500 || (statusCode >= 300 && statusCode < 400) || (statusCode != http.StatusUnauthorized && statusCode != http.StatusForbidden && (statusCode < 200 || statusCode >= 300)) {
		s.mu.Lock()
		if !s.closed && s.flow == flow {
			s.token = ""
			s.checkedToken = token
			s.state = State{
				SignedIn:         false,
				Checking:         false,
				HasStoredSession: true,
				NetworkError:     true,
				Error:            errSessionCheckFailed,
			}
		}
		snapshot := s.state
		s.mu.Unlock()
		return snapshot
	}

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		s.fileMu.Lock()
		s.mu.Lock()
		if !s.closed && s.flow == flow {
			_ = deleteToken(storagePath)
			s.token = ""
			s.checkedToken = ""
			s.state = State{
				SignedIn:         false,
				Checking:         false,
				HasStoredSession: false,
				Error:            errSessionExpired,
			}
		}
		snapshot := s.state
		s.mu.Unlock()
		s.fileMu.Unlock()
		return snapshot
	}

	s.mu.Lock()
	if !s.closed && s.flow == flow {
		s.token = token
		s.checkedToken = token
		s.state = State{
			SignedIn:         true,
			Email:            email,
			HasStoredSession: true,
		}
	}
	snapshot := s.state
	s.mu.Unlock()
	return snapshot
}

func (s *Service) State(ctx context.Context) State {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	snapshot := s.state
	curToken := s.token
	checkedToken := s.checkedToken
	storagePath := s.storagePath
	flow := s.flow
	closed := s.closed
	s.mu.Unlock()

	if closed {
		return snapshot
	}

	if snapshot.SignedIn {
		s.fileMu.Lock()
		diskToken, err := loadToken(storagePath)
		s.fileMu.Unlock()
		if err != nil || diskToken == "" {
			s.mu.Lock()
			if !s.closed && s.flow == flow && s.state.SignedIn {
				s.token = ""
				s.checkedToken = ""
				s.state = State{SignedIn: false}
			}
			snapshot = s.state
			s.mu.Unlock()
			return snapshot
		}
		if diskToken != curToken {
			return s.Restore(ctx)
		}
		return snapshot
	}

	if !snapshot.Waiting && !snapshot.Checking {
		s.fileMu.Lock()
		diskToken, err := loadToken(storagePath)
		s.fileMu.Unlock()
		if err == nil && diskToken != "" {
			if diskToken != checkedToken {
				return s.Restore(ctx)
			}
			return snapshot
		}
		if os.IsNotExist(err) && snapshot.HasStoredSession {
			s.mu.Lock()
			if !s.closed && s.flow == flow && !s.state.SignedIn && !s.state.Waiting && !s.state.Checking {
				s.token = ""
				s.checkedToken = ""
				s.state = State{SignedIn: false}
			}
			snapshot = s.state
			s.mu.Unlock()
		}
	}

	return snapshot
}

func (s *Service) StartLogin(ctx context.Context) (State, error) {
	rawHash := make([]byte, 32)
	if _, err := rand.Read(rawHash); err != nil {
		return s.State(ctx), err
	}
	hash := "desktop_" + hex.EncodeToString(rawHash)

	pollCtx, pollCancel := context.WithCancel(context.Background())

	s.mu.Lock()
	if s.closed {
		snapshot := s.state
		s.mu.Unlock()
		pollCancel()
		return snapshot, errors.New("account service closed")
	}
	s.flow++
	flow := s.flow
	if s.pollCancel != nil {
		s.pollCancel()
	}
	s.pollCancel = pollCancel
	s.token = ""
	s.checkedToken = ""
	loginURL := s.baseURL + "/auth/google?app_hash=" + hash
	s.state = State{
		Waiting:  true,
		Checking: false,
		LoginURL: loginURL,
	}
	openBrowser := s.openBrowser
	s.mu.Unlock()

	var browserErr error
	if openBrowser != nil {
		browserErr = openBrowser(loginURL)
	}

	s.mu.Lock()
	if s.closed {
		snapshot := s.state
		s.mu.Unlock()
		pollCancel()
		return snapshot, errors.New("account service closed")
	}
	if s.flow != flow {
		snapshot := s.state
		s.mu.Unlock()
		pollCancel()
		return snapshot, nil
	}
	if browserErr != nil {
		s.state.Error = errBrowserOpenFailed
	}
	snapshot := s.state
	s.wg.Add(1)
	go s.poll(pollCtx, hash, flow)
	s.mu.Unlock()

	return snapshot, nil
}

func (s *Service) CancelOrLogout(ctx context.Context) State {
	s.mu.Lock()
	s.flow++
	if s.pollCancel != nil {
		s.pollCancel()
		s.pollCancel = nil
	}
	s.token = ""
	s.checkedToken = ""
	s.state = State{}
	storagePath := s.storagePath
	snapshot := s.state
	s.mu.Unlock()

	s.fileMu.Lock()
	s.mu.Lock()
	shouldDelete := !s.state.SignedIn && !s.state.Waiting
	s.mu.Unlock()
	if shouldDelete {
		_ = deleteToken(storagePath)
	}
	s.fileMu.Unlock()

	return snapshot
}

func (s *Service) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.flow++
	if s.pollCancel != nil {
		s.pollCancel()
		s.pollCancel = nil
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *Service) poll(ctx context.Context, hash string, flow uint64) {
	defer s.wg.Done()

	for attempt := 0; attempt < s.maxAttempts; attempt++ {
		timer := time.NewTimer(s.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		s.mu.Lock()
		active := !s.closed && s.flow == flow
		baseURL := s.baseURL
		storagePath := s.storagePath
		s.mu.Unlock()
		if !active {
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/auth/desktop-auth/"+hash, nil)
		if err != nil {
			s.setPollError(flow, errLoginNetwork)
			return
		}
		req.Header.Set("Cache-Control", "no-store")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.setPollError(flow, errLoginNetwork)
			return
		}

		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			s.setPollError(flow, fmt.Sprintf("Сервер входа недоступен (HTTP %d). Повторите попытку.", resp.StatusCode))
			return
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			_ = resp.Body.Close()
			s.setPollError(flow, errLoginNetwork)
			return
		}

		var payload desktopAuthResponse
		_ = json.NewDecoder(resp.Body).Decode(&payload)
		_ = resp.Body.Close()

		s.mu.Lock()
		stillActive := !s.closed && s.flow == flow
		s.mu.Unlock()
		if !stillActive {
			return
		}

		if payload.Status == "success" && payload.Token != "" {
			email := payload.User.Email
			if email == "" {
				email = payload.Email
			}

			s.fileMu.Lock()
			s.mu.Lock()
			if s.closed || s.flow != flow {
				s.mu.Unlock()
				s.fileMu.Unlock()
				return
			}
			if err := saveToken(storagePath, payload.Token); err != nil {
				s.state.Error = fmt.Sprintf("Не удалось сохранить сессию: %v", err)
				s.state.Waiting = false
				s.mu.Unlock()
				s.fileMu.Unlock()
				return
			}
			s.token = payload.Token
			s.checkedToken = payload.Token
			s.state = State{
				SignedIn:         true,
				Email:            email,
				HasStoredSession: true,
			}
			s.mu.Unlock()
			s.fileMu.Unlock()
			return
		}
	}

	s.setPollError(flow, errLoginTimeout)
}

func (s *Service) setPollError(flow uint64, message string) {
	s.mu.Lock()
	if !s.closed && s.flow == flow {
		s.state.Error = message
		s.state.Waiting = false
	}
	s.mu.Unlock()
}

func (s *Service) fetchMe(ctx context.Context, baseURL, token string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/auth/me", nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", resp.StatusCode, nil
	}

	var payload meResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", resp.StatusCode, err
	}
	email := payload.User.Email
	if email == "" {
		email = payload.Email
	}
	return email, resp.StatusCode, nil
}

type meResponse struct {
	User struct {
		Email string `json:"email"`
	} `json:"user"`
	Email string `json:"email"`
}

type desktopAuthResponse struct {
	Status string `json:"status"`
	Token  string `json:"token"`
	User   struct {
		Email string `json:"email"`
	} `json:"user"`
	Email string `json:"email"`
}
