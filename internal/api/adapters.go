package api

import (
	"io"

	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/storage"
)

// ─── Database Adapter ────────────────────────────────────────────────────────
// The db.Engine already matches DatabaseService exactly — no adapter needed.

// WrapAuthService wraps an auth.AuthService to implement api.AuthService.
func WrapAuthService(svc *auth.AuthService) AuthService {
	return &authAdapter{svc: svc}
}

// WrapStorageDriver wraps a storage.Driver to implement api.StorageService.
func WrapStorageDriver(drv storage.Driver) StorageService {
	return &storageAdapter{drv: drv}
}

// ─── Auth Adapter ────────────────────────────────────────────────────────────

type authAdapter struct {
	svc *auth.AuthService
}

func (a *authAdapter) SignUp(email, password string) (*UserInfo, *TokenPair, error) {
	user, tokens, err := a.svc.SignUp(email, password)
	if err != nil {
		return nil, nil, err
	}
	return authUserToAPI(user), authTokensToAPI(tokens), nil
}

func (a *authAdapter) SignIn(email, password string) (*TokenPair, error) {
	tokens, err := a.svc.SignIn(email, password)
	if err != nil {
		return nil, err
	}
	return authTokensToAPI(tokens), nil
}

func (a *authAdapter) RefreshToken(refreshToken string) (*TokenPair, error) {
	tokens, err := a.svc.RefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}
	return authTokensToAPI(tokens), nil
}

func (a *authAdapter) ValidateAccessToken(tokenString string) (*UserClaims, error) {
	claims, err := a.svc.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, err
	}
	return &UserClaims{
		UserID: claims.UserID,
		Email:  claims.Email,
		Role:   claims.Role,
	}, nil
}

func (a *authAdapter) GetUser(id string) (*UserInfo, error) {
	user, err := a.svc.GetUser(id)
	if err != nil {
		return nil, err
	}
	return authUserToAPI(user), nil
}

func (a *authAdapter) CreateOAuthState(provider, projectID, appRedirect string) (string, error) {
	return a.svc.CreateOAuthState(provider, projectID, appRedirect)
}

func (a *authAdapter) CreateOAuthStateURL(provider, projectID, appRedirect string) (string, string, error) {
	return a.svc.CreateOAuthStateURL(provider, projectID, appRedirect)
}

func (a *authAdapter) DecodeStatePayload(state string) (*auth.OAuthStatePayload, error) {
	return a.svc.DecodeStatePayload(state)
}


func (a *authAdapter) HandleOAuthCallback(provider, code, state string) (*UserInfo, *TokenPair, error) {
	user, tokens, err := a.svc.HandleOAuthCallback(provider, code, state)
	if err != nil {
		return nil, nil, err
	}
	return authUserToAPI(user), authTokensToAPI(tokens), nil
}

func (a *authAdapter) VerifyEmail(token string) error {
	return a.svc.VerifyEmail(token)
}

func (a *authAdapter) ForgotPassword(email string) (string, error) {
	return a.svc.ForgotPassword(email)
}

func (a *authAdapter) ResetPassword(token, newPassword string) error {
	return a.svc.ResetPassword(token, newPassword)
}

func (a *authAdapter) CreateMagicLink(email string) (string, error) {
	return a.svc.CreateMagicLink(email)
}

func (a *authAdapter) VerifyMagicLink(email, token string) (*TokenPair, error) {
	tokens, err := a.svc.VerifyMagicLink(email, token)
	if err != nil {
		return nil, err
	}
	return authTokensToAPI(tokens), nil
}

func (a *authAdapter) SetupMFA(userID string) (string, string, error) {
	return a.svc.SetupMFA(userID)
}

func (a *authAdapter) ConfirmMFA(userID, code string) ([]string, error) {
	return a.svc.ConfirmMFA(userID, code)
}

func (a *authAdapter) DisableMFA(userID, code string) error {
	return a.svc.DisableMFA(userID, code)
}

func (a *authAdapter) VerifyMFA(userID, code string) error {
	return a.svc.VerifyMFA(userID, code)
}

func (a *authAdapter) GetMFAStatus(userID string) (bool, error) {
	return a.svc.GetMFAStatus(userID)
}

func authUserToAPI(u *auth.User) *UserInfo {
	if u == nil {
		return nil
	}
	providers := make([]ProviderMetaInfo, len(u.OAuthProviders))
	for i, p := range u.OAuthProviders {
		providers[i] = ProviderMetaInfo{
			Provider:   p.Provider,
			ProviderID: p.ProviderID,
		}
	}
	return &UserInfo{
		ID:                u.ID,
		Email:             u.Email,
		Role:              string(u.Role),
		Name:              u.Name,
		AvatarURL:         u.AvatarURL,
		OAuthProviders:    providers,
		CreatedAt:         u.CreatedAt,
		IsVerified:        u.IsVerified,
		VerificationToken: u.VerificationToken,
	}
}

func authTokensToAPI(t *auth.TokenPair) *TokenPair {
	if t == nil {
		return nil
	}
	return &TokenPair{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresIn:    int(t.ExpiresIn),
	}
}

// ─── Storage Adapter ─────────────────────────────────────────────────────────

type storageAdapter struct {
	drv storage.Driver
}

func (s *storageAdapter) Upload(bucket, path string, reader io.Reader, contentType string) (*FileInfo, error) {
	info, err := s.drv.Upload(bucket, path, reader, contentType)
	if err != nil {
		return nil, err
	}
	return storageFileInfoToAPI(info), nil
}

func (s *storageAdapter) Download(bucket, path string) (io.ReadCloser, *FileInfo, error) {
	rc, info, err := s.drv.Download(bucket, path)
	if err != nil {
		return nil, nil, err
	}
	return rc, storageFileInfoToAPI(info), nil
}

func (s *storageAdapter) Delete(bucket, path string) error {
	return s.drv.Delete(bucket, path)
}

func (s *storageAdapter) List(bucket, prefix string) ([]FileInfo, error) {
	infos, err := s.drv.List(bucket, prefix)
	if err != nil {
		return nil, err
	}
	result := make([]FileInfo, len(infos))
	for i, info := range infos {
		result[i] = *storageFileInfoToAPI(&info)
	}
	return result, nil
}

func storageFileInfoToAPI(info *storage.FileInfo) *FileInfo {
	return &FileInfo{
		Bucket:      info.Bucket,
		Path:        info.Path,
		Size:        info.Size,
		ContentType: info.ContentType,
		CreatedAt:   info.CreatedAt,
		UpdatedAt:   info.UpdatedAt,
		URL:         info.URL,
	}
}
