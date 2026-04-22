package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"hookforward/backend/internal/auth"
	"hookforward/backend/internal/config"
	"hookforward/backend/internal/domain"
	"hookforward/backend/internal/mailer"
	"hookforward/backend/internal/repository"
	"hookforward/backend/internal/verification"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUserDisabled        = errors.New("user disabled")
	ErrEmailAlreadyUsed    = errors.New("email already used")
	ErrCodeSendTooFast     = errors.New("verification code requested too frequently")
	ErrInvalidCode         = errors.New("invalid verification code")
	ErrGitHubOAuthDisabled = errors.New("github oauth disabled")
	ErrInvalidOAuthState   = errors.New("invalid oauth state")
)

type AuthService struct {
	users       *repository.UserRepository
	providers   *repository.UserAuthProviderRepository
	tokens      *auth.TokenIssuer
	verifyStore *verification.Store
	emailSender mailer.EmailSender
	env         string
	cfg         config.Config
	httpClient  *http.Client
}

type UserView struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	EmailVerified bool       `json:"emailVerified"`
	AuthSource    string     `json:"authSource"`
	DisplayName   string     `json:"displayName"`
	AvatarURL     string     `json:"avatarUrl"`
	Role          string     `json:"role"`
	Status        string     `json:"status"`
	LastLoginAt   *time.Time `json:"lastLoginAt"`
}

type LoginResult struct {
	Token     string    `json:"accessToken"`
	ExpiresAt time.Time `json:"expiresAt"`
	User      UserView  `json:"user"`
}

func NewAuthService(users *repository.UserRepository, providers *repository.UserAuthProviderRepository, jwtSecret string, verifyStore *verification.Store, emailSender mailer.EmailSender, cfg config.Config) *AuthService {
	return &AuthService{
		users:       users,
		providers:   providers,
		tokens:      auth.NewTokenIssuer(jwtSecret),
		verifyStore: verifyStore,
		emailSender: emailSender,
		env:         cfg.Env,
		cfg:         cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type SendCodeResult struct {
	Sent      bool   `json:"sent"`
	DebugCode string `json:"debugCode,omitempty"`
}

func (s *AuthService) SendRegisterCode(ctx context.Context, email string) (SendCodeResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return SendCodeResult{}, ErrEmailAlreadyUsed
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		return SendCodeResult{}, err
	}

	return s.sendCode(ctx, "register", email)
}

func (s *AuthService) Register(ctx context.Context, email string, code string, password string) (LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return LoginResult{}, ErrEmailAlreadyUsed
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		return LoginResult{}, err
	}

	if err := auth.ValidatePasswordStrength(password); err != nil {
		return LoginResult{}, err
	}

	if err := s.requireCode(ctx, "register", email, code); err != nil {
		return LoginResult{}, err
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return LoginResult{}, err
	}

	now := time.Now().UTC()
	user := domain.User{
		ID:            newUserID(),
		Email:         email,
		EmailVerified: true,
		PasswordHash:  passwordHash,
		AuthSource:    "email",
		DisplayName:   strings.Split(email, "@")[0],
		AvatarURL:     "",
		Role:          "user",
		Status:        "active",
		LastLoginAt:   &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.users.Insert(ctx, user); err != nil {
		return LoginResult{}, err
	}
	_ = s.verifyStore.DeleteCode(ctx, "register", email)

	token, expiresAt, err := s.tokens.Issue(user.ID, user.Email, user.Role)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      userViewFromDomain(user),
	}, nil
}

func (s *AuthService) SendResetCode(ctx context.Context, email string) (SendCodeResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := s.users.FindByEmail(ctx, email); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return SendCodeResult{Sent: true}, nil
		}
		return SendCodeResult{}, err
	}

	return s.sendCode(ctx, "reset", email)
}

func (s *AuthService) ResetPassword(ctx context.Context, email string, code string, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := s.users.FindByEmail(ctx, email); err != nil {
		return err
	}

	if err := auth.ValidatePasswordStrength(password); err != nil {
		return err
	}

	if err := s.requireCode(ctx, "reset", email, code); err != nil {
		return err
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	if err := s.users.UpdatePasswordByEmail(ctx, email, passwordHash, time.Now().UTC()); err != nil {
		return err
	}
	_ = s.verifyStore.DeleteCode(ctx, "reset", email)
	return nil
}

func (s *AuthService) Login(ctx context.Context, email string, password string) (LoginResult, error) {
	user, err := s.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}

	if user.Status != "active" {
		return LoginResult{}, ErrUserDisabled
	}

	if err := auth.VerifyPassword(user.PasswordHash, password); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	token, expiresAt, err := s.tokens.Issue(user.ID, user.Email, user.Role)
	if err != nil {
		return LoginResult{}, err
	}

	now := time.Now().UTC()
	if err := s.users.TouchLastLogin(ctx, user.ID, now); err != nil {
		return LoginResult{}, err
	}
	user.LastLoginAt = &now

	return LoginResult{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      userViewFromDomain(user),
	}, nil
}

func (s *AuthService) GitHubAuthURL(ctx context.Context) (string, string, error) {
	if strings.TrimSpace(s.cfg.GitHubClientID) == "" || strings.TrimSpace(s.cfg.GitHubClientSecret) == "" {
		return "", "", ErrGitHubOAuthDisabled
	}
	if s.verifyStore == nil {
		return "", "", errors.New("verification store unavailable")
	}

	state, err := randomToken(24)
	if err != nil {
		return "", "", err
	}

	if err := s.verifyStore.SaveCode(ctx, "oauth-github-state", state, state, 10*time.Minute); err != nil {
		return "", "", err
	}

	params := url.Values{}
	params.Set("client_id", s.cfg.GitHubClientID)
	params.Set("redirect_uri", strings.TrimRight(s.cfg.PublicBaseURL, "/")+"/api/v1/auth/github/callback")
	params.Set("scope", "read:user user:email")
	params.Set("state", state)

	return "https://github.com/login/oauth/authorize?" + params.Encode(), state, nil
}

func (s *AuthService) CompleteGitHubLogin(ctx context.Context, state string, code string) (LoginResult, error) {
	if strings.TrimSpace(s.cfg.GitHubClientID) == "" || strings.TrimSpace(s.cfg.GitHubClientSecret) == "" {
		return LoginResult{}, ErrGitHubOAuthDisabled
	}
	if strings.TrimSpace(state) == "" || strings.TrimSpace(code) == "" {
		return LoginResult{}, ErrInvalidOAuthState
	}
	if s.verifyStore == nil {
		return LoginResult{}, errors.New("verification store unavailable")
	}

	storedState, err := s.verifyStore.LoadCode(ctx, "oauth-github-state", state)
	if err != nil || storedState != state {
		return LoginResult{}, ErrInvalidOAuthState
	}
	_ = s.verifyStore.DeleteCode(ctx, "oauth-github-state", state)

	tokenResponse, err := s.exchangeGitHubCode(ctx, code)
	if err != nil {
		return LoginResult{}, err
	}

	profile, err := s.fetchGitHubProfile(ctx, tokenResponse.AccessToken)
	if err != nil {
		return LoginResult{}, err
	}

	email, emailVerified, err := s.resolveGitHubEmail(ctx, tokenResponse.AccessToken, profile)
	if err != nil {
		return LoginResult{}, err
	}

	user, err := s.findOrCreateGitHubUser(ctx, profile, email, emailVerified)
	if err != nil {
		return LoginResult{}, err
	}

	now := time.Now().UTC()
	if err := s.providers.Upsert(ctx, domain.UserAuthProvider{
		ID:                    repository.NewUserAuthProviderID(),
		UserID:                user.ID,
		Provider:              "github",
		ProviderUserID:        fmt.Sprintf("%d", profile.ID),
		ProviderUsername:      profile.Login,
		ProviderEmail:         email,
		AccessTokenEncrypted:  "",
		RefreshTokenEncrypted: "",
		TokenExpiresAt:        nil,
		Scope:                 tokenResponse.Scope,
		ProfileJSON:           profile.RawJSON,
		CreatedAt:             now,
		UpdatedAt:             now,
	}); err != nil {
		return LoginResult{}, err
	}

	token, expiresAt, err := s.tokens.Issue(user.ID, user.Email, user.Role)
	if err != nil {
		return LoginResult{}, err
	}

	if err := s.users.TouchLastLogin(ctx, user.ID, now); err != nil {
		return LoginResult{}, err
	}
	user.LastLoginAt = &now

	return LoginResult{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      userViewFromDomain(user),
	}, nil
}

func (s *AuthService) sendCode(ctx context.Context, purpose string, email string) (SendCodeResult, error) {
	if s.verifyStore == nil {
		return SendCodeResult{}, errors.New("verification store unavailable")
	}

	allowed, err := s.verifyStore.AllowSend(ctx, purpose, email, 60*time.Second)
	if err != nil {
		return SendCodeResult{}, err
	}
	if !allowed {
		return SendCodeResult{}, ErrCodeSendTooFast
	}

	code, err := generateCode()
	if err != nil {
		return SendCodeResult{}, err
	}
	if err := s.verifyStore.SaveCode(ctx, purpose, email, code, 5*time.Minute); err != nil {
		return SendCodeResult{}, err
	}

	if s.emailSender != nil {
		if err := s.emailSender.SendVerificationCode(ctx, purpose, email, code); err != nil {
			return SendCodeResult{}, err
		}
	} else if s.env != "development" {
		return SendCodeResult{}, errors.New("email delivery unavailable")
	}

	result := SendCodeResult{Sent: true}
	if s.env == "development" {
		result.DebugCode = code
	}
	return result, nil
}

func (s *AuthService) requireCode(ctx context.Context, purpose string, email string, code string) error {
	if s.verifyStore == nil {
		return errors.New("verification store unavailable")
	}

	stored, err := s.verifyStore.LoadCode(ctx, purpose, email)
	if err != nil {
		return ErrInvalidCode
	}
	if strings.TrimSpace(code) != stored {
		return ErrInvalidCode
	}
	return nil
}

func generateCode() (string, error) {
	max := big.NewInt(1000000)
	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func newUserID() string {
	return "usr_" + newMessageID(8)
}

type gitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
}

type gitHubProfile struct {
	ID        int64           `json:"id"`
	Login     string          `json:"login"`
	Name      string          `json:"name"`
	AvatarURL string          `json:"avatar_url"`
	Email     string          `json:"email"`
	RawJSON   json.RawMessage `json:"-"`
}

type gitHubEmail struct {
	Email      string `json:"email"`
	Primary    bool   `json:"primary"`
	Verified   bool   `json:"verified"`
	Visibility string `json:"visibility"`
}

func (s *AuthService) exchangeGitHubCode(ctx context.Context, code string) (gitHubTokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", s.cfg.GitHubClientID)
	form.Set("client_secret", s.cfg.GitHubClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", strings.TrimRight(s.cfg.PublicBaseURL, "/")+"/api/v1/auth/github/callback")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return gitHubTokenResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return gitHubTokenResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return gitHubTokenResponse{}, err
	}

	var payload gitHubTokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return gitHubTokenResponse{}, err
	}
	if resp.StatusCode >= 400 || payload.AccessToken == "" {
		if payload.Error != "" {
			return gitHubTokenResponse{}, errors.New(payload.Error)
		}
		return gitHubTokenResponse{}, errors.New("github token exchange failed")
	}

	return payload, nil
}

func (s *AuthService) fetchGitHubProfile(ctx context.Context, accessToken string) (gitHubProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return gitHubProfile{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", s.cfg.AppName)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return gitHubProfile{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return gitHubProfile{}, err
	}
	if resp.StatusCode >= 400 {
		return gitHubProfile{}, errors.New("failed to fetch github profile")
	}

	var profile gitHubProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return gitHubProfile{}, err
	}
	profile.RawJSON = append(profile.RawJSON[:0], body...)
	return profile, nil
}

func (s *AuthService) resolveGitHubEmail(ctx context.Context, accessToken string, profile gitHubProfile) (string, bool, error) {
	if strings.TrimSpace(profile.Email) != "" {
		return strings.ToLower(strings.TrimSpace(profile.Email)), true, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", s.cfg.AppName)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}
	if resp.StatusCode >= 400 {
		return "", false, errors.New("failed to fetch github emails")
	}

	var emails []gitHubEmail
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", false, err
	}

	for _, item := range emails {
		if item.Primary && item.Verified && strings.TrimSpace(item.Email) != "" {
			return strings.ToLower(strings.TrimSpace(item.Email)), true, nil
		}
	}
	for _, item := range emails {
		if item.Verified && strings.TrimSpace(item.Email) != "" {
			return strings.ToLower(strings.TrimSpace(item.Email)), true, nil
		}
	}

	login := strings.TrimSpace(profile.Login)
	if login == "" {
		login = fmt.Sprintf("github-%d", profile.ID)
	}
	return strings.ToLower(login) + fmt.Sprintf("+%d@users.noreply.hookforward.local", profile.ID), false, nil
}

func (s *AuthService) findOrCreateGitHubUser(ctx context.Context, profile gitHubProfile, email string, emailVerified bool) (domain.User, error) {
	providerUserID := fmt.Sprintf("%d", profile.ID)
	if s.providers != nil {
		providerLink, err := s.providers.FindByProviderAndProviderUserID(ctx, "github", providerUserID)
		if err == nil {
			user, getErr := s.users.FindByID(ctx, providerLink.UserID)
			if getErr == nil {
				if user.Status != "active" {
					return domain.User{}, ErrUserDisabled
				}
				return user, nil
			}
			if !errors.Is(getErr, repository.ErrUserNotFound) {
				return domain.User{}, getErr
			}
		} else if !errors.Is(err, repository.ErrUserAuthProviderNotFound) {
			return domain.User{}, err
		}
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err == nil {
		if user.Status != "active" {
			return domain.User{}, ErrUserDisabled
		}
		return user, nil
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return domain.User{}, err
	}

	now := time.Now().UTC()
	displayName := strings.TrimSpace(profile.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(profile.Login)
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}

	user = domain.User{
		ID:            newUserID(),
		Email:         email,
		EmailVerified: emailVerified,
		PasswordHash:  "",
		AuthSource:    "github",
		DisplayName:   displayName,
		AvatarURL:     profile.AvatarURL,
		Role:          "user",
		Status:        "active",
		LastLoginAt:   &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.users.Insert(ctx, user); err != nil {
		return domain.User{}, err
	}

	return user, nil
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		size = 24
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
