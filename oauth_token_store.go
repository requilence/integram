package integram

import (
	"fmt"
	"time"

	"gopkg.in/mgo.v2/bson"
)

type OAuthTokenStore interface {
	Name() string
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

func MigrateOAuthFromTo(c *Context, oldTS OAuthTokenStore, newTS OAuthTokenStore, onlyValid bool) (total int, migrated int, expired int, err error) {
	keyPrefix := "protected." + c.ServiceName

	query := bson.M{
		keyPrefix + ".oauthstore": oldTS.Name(),
	}

	if onlyValid {
		query[keyPrefix+".oauthvalid"] = true
	}

	users, err := c.FindUsers(query)
	if err != nil {
		return
	}

	total = len(users)
	expiredOlderThan := time.Now().Add((-1) * time.Hour * 24 * 30)
	for i, userData := range users {
		ctxCopy := *userData.ctx
		ctxCopy.User = userData.User
		ctxCopy.User.ctx = &ctxCopy
		ctxCopy.Chat = Chat{ID: userData.ID, ctx: &ctxCopy}
		userData.ctx = &ctxCopy
		user := userData.User
		user.data = &userData

		if i%100 == 0 {
			fmt.Printf("MigrateOAuthFromTo: %d/%d users transfered\n", i, len(users))
		}

		token, expiry, err := oldTS.GetOAuthAccessToken(&user)
		if err != nil {
			c.Log().Errorf("MigrateOAuthFromTo got error on GetOAuthAccessToken: %s", err.Error())
			continue
		}

		if onlyValid && token == "" {
			expired++
			continue
		}

		if onlyValid && expiry != nil && expiry.Before(expiredOlderThan) {
			expired++
			continue
		}

		err = newTS.SetOAuthAccessToken(&user, token, expiry)
		if err != nil {
			c.Log().Errorf("MigrateOAuthFromTo got error on SetOAuthAccessToken: %s", err.Error())
			continue
		}

		refreshToken, err := oldTS.GetOAuthRefreshToken(&user)
		if err != nil {
			c.Log().Errorf("MigrateOAuthFromTo got error on GetOAuthRefreshToken: %s", err.Error())
			continue
		}

		err = newTS.SetOAuthRefreshToken(&user, refreshToken)
		if err != nil {
			c.Log().Errorf("MigrateOAuthFromTo got error on SetOAuthRefreshToken: %s", err.Error())
			continue
		}

		err = c.db.C("users").UpdateId(user.ID, bson.M{"$set": bson.M{keyPrefix + ".oauthstore": newTS.Name(), keyPrefix + ".oauthvalid": true}})
		if err != nil {
			c.Log().Errorf("MigrateOAuthFromTo got error: %s", err.Error())
			continue
		}

		migrated++
	}

	fmt.Printf("MigrateOAuthFromTo: %d/%d users transfered\n", len(users), len(users))

	return
}

func (d *DefaultOAuthTokenMongoStore) GetOAuthAccessToken(user *User) (token string, expireDate *time.Time, err error) {
	ps, err := user.protectedSettings()
	if err != nil {
		return "", nil, err
	}

	return ps.OAuthToken, ps.OAuthExpireDate, nil
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

	ps.OAuthStore = d.Name()
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

func (d *DefaultOAuthTokenMongoStore) Name() string {
	return "default"
}
