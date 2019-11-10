package integram

import (
	"time"
)

type OAuthTokenStore interface {
	GetOAuthAccessToken(user *User) (token string, expireDate *time.Time, err error)
	SetOAuthAccessToken(user *User, token string, expireDate *time.Time) error
	GetOAuthRefreshToken(user *User) (string, error)
	SetOAuthRefreshToken(user *User, token string) error
}

type DefaultOAuthTokenMongoStore struct {
}

var oauthTokenStore OAuthTokenStore = &DefaultOAuthTokenMongoStore{}

func SetOAuthTokenStore(store OAuthTokenStore) {
	oauthTokenStore = store
}

func (d *DefaultOAuthTokenMongoStore) GetOAuthAccessToken(user *User) (token string, expireDate *time.Time, err error) {
	ps, err := user.protectedSettings()
	if err != nil {
		return "", nil, err
	}

	return ps.OAuthToken, nil, nil
}

func (d *DefaultOAuthTokenMongoStore) GetOAuthRefreshToken(user *User) (string, error) {
	ps, err := user.protectedSettings()

	if err != nil {
		return "", err
	}

	return ps.OAuthRefreshToken, nil
}

func (d *DefaultOAuthTokenMongoStore) SetOAuthAccessToken(user *User, token string, expireDate *time.Time) error {
	ps, err := user.protectedSettings()
	if err != nil {
		return err
	}

	ps.OAuthToken = token
	ps.OAuthExpireDate = expireDate

	return user.saveProtectedSettings()
}

func (d *DefaultOAuthTokenMongoStore) SetOAuthRefreshToken(user *User, refreshToken string) error {
	ps, err := user.protectedSettings()
	if err != nil {
		return err
	}

	ps.OAuthRefreshToken = refreshToken

	return user.saveProtectedSetting("OAuthRefreshToken", refreshToken)
}
