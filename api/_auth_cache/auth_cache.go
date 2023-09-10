package _auth_cache

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/matrix"
)

var tokenCache = cache.New(cache.NoExpiration, 30*time.Second)
var rwLock = &sync.RWMutex{}
var regexCache = make(map[string]*regexp.Regexp)

type cachedToken struct {
	userId string
	err    error
}

func cacheKey(accessToken string, appserviceUserId string) string {
	if appserviceUserId != "" {
		return fmt.Sprintf("@@%s@@%s", accessToken, appserviceUserId)
	}
	return fmt.Sprintf("@@%s@@__NOOP__", accessToken)
}

func FlushCache() {
	rwLock.Lock()
	tokenCache.Flush()
	rwLock.Unlock()
}

func InvalidateToken(ctx rcontext.RequestContext, accessToken string, appserviceUserId string) error {
	if ctx.Request == nil {
		ctx.Log.Warn("Tried to logout without a valid request reference")
		return errors.New("invalid context - missing request")
	}

	err := matrix.Logout(ctx, ctx.Request.Host, accessToken, appserviceUserId, ctx.Request.RemoteAddr)
	if err != nil {
		return err
	}

	rwLock.Lock()
	tokenCache.Delete(cacheKey(accessToken, appserviceUserId))
	tokenCache.Delete(cacheKey(accessToken, ""))
	rwLock.Unlock()
	return nil
}

func InvalidateAllTokens(ctx rcontext.RequestContext, accessToken string, appserviceUserId string) error {
	if ctx.Request == nil {
		ctx.Log.Warn("Tried to logout without a valid request reference")
		return errors.New("invalid context - missing request")
	}

	err := matrix.LogoutAll(ctx, ctx.Request.Host, accessToken, appserviceUserId, ctx.Request.RemoteAddr)
	if err != nil {
		return err
	}

	rwLock.Lock()
	// It's safer to flush the whole cache instead of iterating over thousands of tokens
	tokenCache.Flush()
	rwLock.Unlock()
	return nil
}

func GetUserId(ctx rcontext.RequestContext, accessToken string, appserviceUserId string) (string, error) {
	if ctx.Request == nil {
		ctx.Log.Warn("Tried to get user ID for access token without a valid request reference")
		return "", errors.New("invalid context - missing request")
	}

	if accessToken == "" {
		ctx.Log.Warn("No access token supplied - cannot get user ID")
		return "", matrix.ErrInvalidToken
	}

	if ctx.Config.AccessTokens.MaxCacheTimeSeconds <= 0 {
		ctx.Log.Warn("Access token cache is disabled for this host")
		return checkTokenWithHomeserver(ctx, accessToken, appserviceUserId, false)
	}

	rwLock.Lock()
	record, ok := tokenCache.Get(cacheKey(accessToken, appserviceUserId))
	rwLock.Unlock()
	if ok {
		token := record.(cachedToken)
		if token.err != nil {
			return "", token.err
		}
		ctx.Log.Debugf("Access token belongs to %s", token.userId)
		return token.userId, nil
	}

	if !ctx.Config.AccessTokens.UseAppservices {
		// We pass the appservice user ID through to the homeserver as it might know what is going on
		return checkTokenWithHomeserver(ctx, accessToken, appserviceUserId, true)
	}

	for _, r := range ctx.Config.AccessTokens.Appservices {
		if r.AppserviceToken != accessToken {
			continue
		}

		if r.SenderUserId != "" && (r.SenderUserId == appserviceUserId || appserviceUserId == "") {
			ctx.Log.Debugf("Access token belongs to appservice (sender user ID): %s", r.Id)
			cacheToken(ctx, accessToken, appserviceUserId, r.SenderUserId, nil)
			return r.SenderUserId, nil
		}

		for _, n := range r.UserNamespaces {
			regex, ok := regexCache[n.Regex]
			if !ok {
				regex = regexp.MustCompile(n.Regex)
				regexCache[n.Regex] = regex
			}
			if regex.MatchString(appserviceUserId) {
				ctx.Log.Debugf("Access token belongs to appservice: %s", r.Id)
				cacheToken(ctx, accessToken, appserviceUserId, appserviceUserId, nil)
				return appserviceUserId, nil
			}
		}
	}

	// We pass the appservice user ID through to the homeserver as it might know what is going on
	return checkTokenWithHomeserver(ctx, accessToken, appserviceUserId, true)
}

func cacheToken(ctx rcontext.RequestContext, accessToken string, appserviceUserId string, userId string, err error) {
	v := cachedToken{
		userId: userId,
		err:    err,
	}
	t := time.Duration(ctx.Config.AccessTokens.MaxCacheTimeSeconds) * time.Second
	rwLock.Lock()
	tokenCache.Set(cacheKey(accessToken, appserviceUserId), v, t)
	rwLock.Unlock()
}

func checkTokenWithHomeserver(ctx rcontext.RequestContext, accessToken string, appserviceUserId string, withCache bool) (string, error) {
	ctx.Log.Debug("Checking access token with homeserver")
	hsUserId, err := matrix.GetUserIdFromToken(ctx, ctx.Request.Host, accessToken, appserviceUserId, ctx.Request.RemoteAddr)
	if withCache {
		ctx.Log.Debug("Caching access token response from homeserver")
		cacheToken(ctx, accessToken, appserviceUserId, hsUserId, err)
	}
	return hsUserId, err
}
