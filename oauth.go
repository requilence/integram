package integram

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type OAuthTokenSource struct {
	user *User
	last oauth2.Token
}

func (tsw *OAuthTokenSource) Token() (*oauth2.Token, error) {
	lastToken := tsw.last
	ts := tsw.user.ctx.OAuthProvider().OAuth2Client(tsw.user.ctx).TokenSource(oauth2.NoContext, &lastToken)
	token, err := ts.Token()
	if err != nil {
		if strings.Contains(err.Error(), "revoked") || strings.Contains(err.Error(), "invalid_grant") {
			_ = tsw.user.saveProtectedSetting("OAuthValid", false)

			//todo: provide revoked callback
		}
		tsw.user.ctx.Log().Errorf("OAuth token refresh failed, token OAuthValid set to false: %s", err.Error())
		return nil, err
	}

	if token != nil {
		if token.AccessToken != lastToken.AccessToken || !token.Expiry.Equal(lastToken.Expiry) {
			ps, _ := tsw.user.protectedSettings()
			if ps != nil && !ps.OAuthValid {
				ps.OAuthValid = true
				_ = tsw.user.saveProtectedSetting("OAuthValid", true)
			}

			err = oauthTokenStore.SetOAuthAccessToken(tsw.user, token.AccessToken, &token.Expiry)
			if err != nil {
				tsw.user.ctx.Log().Errorf("failed to set OAuth Access token in store: %s", err.Error())
			}
		}
		if token.RefreshToken != "" && token.RefreshToken != lastToken.RefreshToken {
			err = oauthTokenStore.SetOAuthRefreshToken(tsw.user, token.RefreshToken)
			if err != nil {
				tsw.user.ctx.Log().Errorf("failed to set OAuth Access token in store: %s", err.Error())
			}
		}
	}

	return token, nil
}

// OAuthHTTPClient returns HTTP client with Bearer authorization headers
func (user *User) OAuthHTTPClient() *http.Client {
	if user.ctx.Service().DefaultOAuth2 != nil {
		ts, err := user.OAuthTokenSource()
		if err != nil {
			user.ctx.Log().Errorf("OAuthTokenSource got error: %s", err.Error())
			return nil
		}

		return oauth2.NewClient(oauth2.NoContext, ts)
	} else if user.ctx.Service().DefaultOAuth1 != nil {
		//todo make a correct httpclient
		return http.DefaultClient
	}
	return nil
}

// OAuthValid checks if OAuthToken for service is set
func (user *User) OAuthValid() bool {
	token, _, _ := oauthTokenStore.GetOAuthAccessToken(user)
	return token != ""
}

// OAuthTokenStore returns current OAuthTokenStore name used to get/set access and refresh tokens
func (user *User) OAuthTokenStore() string {
	ps, _ := user.protectedSettings()
	if ps == nil {
		return ""
	}

	return ps.OAuthStore
}

// SetOAuthTokenStore stores the new name for OAuth Store to get/set access and refresh tokens
func (user *User) SetOAuthTokenStore(storeName string)  error {
	return user.saveProtectedSetting("OAuthStore", storeName)
}

// OAuthTokenSource returns OAuthTokenSource to use within http client to get OAuthToken
func (user *User) OAuthTokenSource() (oauth2.TokenSource, error) {
	if user.ctx.Service().DefaultOAuth2 == nil {
		return nil, fmt.Errorf("DefaultOAuth2 config not set for the service")
	}

	accessToken, expireDate, err := oauthTokenStore.GetOAuthAccessToken(user)
	if err != nil {
		user.ctx.Log().Errorf("can't create OAuthTokenSource: oauthTokenStore.GetOAuthAccessToken got error: %s", err.Error())
		return nil, err
	}

	refreshToken, err := oauthTokenStore.GetOAuthRefreshToken(user)
	if err != nil {
		user.ctx.Log().Errorf("can't create OAuthTokenSource: oauthTokenStore.GetOAuthRefreshToken got error: %s", err.Error())
		return nil, err
	}

	otoken := oauth2.Token{AccessToken: accessToken, RefreshToken: refreshToken, TokenType: "Bearer"}
	if expireDate != nil {
		otoken.Expiry = *expireDate
	}

	return &OAuthTokenSource{
		user: user,
		last: otoken,
	}, nil
}

// OAuthToken returns OAuthToken for service
func (user *User) OAuthToken() string {
	// todo: oauthtoken per host?
	/*
		host := user.ctx.ServiceBaseURL.Host

		if host == "" {
			host = user.ctx.Service().DefaultBaseURL.Host
		}
	*/
	ts, err := user.OAuthTokenSource()
	if err != nil {
		user.ctx.Log().Errorf("OAuthTokenSource got error: %s", err.Error())
		return ""
	}

	token, err := ts.Token()
	if err != nil {
		user.ctx.Log().Errorf("OAuthToken got tokensource error: %s", err.Error())
		return ""
	}

	return token.AccessToken
}

// ResetOAuthToken reset OAuthToken for service
func (user *User) ResetOAuthToken() error {
	err := oauthTokenStore.SetOAuthAccessToken(user, "", nil)
	if err != nil {
		user.ctx.Log().WithError(err).Error("ResetOAuthToken error")
	}
	return err
}

// OauthRedirectURL used in OAuth process as returning URL
func (user *User) OauthRedirectURL() string {
	providerID := user.ctx.OAuthProvider().internalID()
	if providerID == user.ctx.ServiceName {
		return fmt.Sprintf("%s/auth/%s", Config.BaseURL, user.ctx.ServiceName)
	}

	return fmt.Sprintf("%s/auth/%s/%s", Config.BaseURL, user.ctx.ServiceName, providerID)
}

// OauthInitURL used in OAuth process as returning URL
func (user *User) OauthInitURL() string {
	authTempToken := user.AuthTempToken()
	s := user.ctx.Service()
	if authTempToken == "" {
		user.ctx.Log().Error("authTempToken is empty")
		return ""
	}
	if s.DefaultOAuth2 != nil {
		provider := user.ctx.OAuthProvider()

		return provider.OAuth2Client(user.ctx).AuthCodeURL(authTempToken, oauth2.AccessTypeOffline)
	}
	if s.DefaultOAuth1 != nil {
		return fmt.Sprintf("%s/oauth1/%s/%s", Config.BaseURL, s.Name, authTempToken)
	}
	return ""
}

// AuthTempToken returns Auth token used in OAuth process to associate TG user with OAuth creditianals
func (user *User) AuthTempToken() string {

	host := user.ctx.ServiceBaseURL.Host
	if host == "" {
		host = user.ctx.Service().DefaultBaseURL.Host
	}

	serviceBaseURL := user.ctx.ServiceBaseURL.String()
	if serviceBaseURL == "" {
		serviceBaseURL = user.ctx.Service().DefaultBaseURL.String()
	}

	ps, _ := user.protectedSettings()
	cacheTime := user.ctx.Service().DefaultOAuth2.AuthTempTokenCacheTime

	if cacheTime == 0 {
		cacheTime = time.Hour * 24 * 30
	}

	if ps.AuthTempToken != "" {
		oAuthIDCacheFound := oAuthIDCacheVal{BaseURL: serviceBaseURL}
		user.SetCache("auth_"+ps.AuthTempToken, oAuthIDCacheFound, cacheTime)

		return ps.AuthTempToken
	}

	rnd := strings.ToLower(rndStr.Get(16))
	user.SetCache("auth_"+rnd, oAuthIDCacheVal{BaseURL: serviceBaseURL}, cacheTime)

	err := user.saveProtectedSetting("AuthTempToken", rnd)

	if err != nil {
		user.ctx.Log().WithError(err).Error("Error saving AuthTempToken")
	}
	return rnd
}

func findOauthProviderByID(db *mgo.Database, id string) (*OAuthProvider, error) {
	oap := OAuthProvider{}

	if s, _ := serviceByName(id); s != nil {
		return s.DefaultOAuthProvider(), nil
	}

	err := db.C("oauth_providers").FindId(id).One(&oap)
	if err != nil {
		return nil, err
	}

	return &oap, nil
}

func findOauthProviderByHost(db *mgo.Database, host string) (*OAuthProvider, error) {
	oap := OAuthProvider{}
	err := db.C("oauth_providers").Find(bson.M{"baseurl.host": strings.ToLower(host)}).One(&oap)
	if err != nil {
		return nil, err
	}

	return &oap, nil
}
