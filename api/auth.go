package api

import (
	"github.com/getsentry/sentry-go"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/auth_cache"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/util"
)

type UserInfo struct {
	UserId      string
	AccessToken string
	IsShared    bool
}

func callUserNext(next func(r *http.Request, rctx rcontext.RequestContext, user UserInfo) interface{}, r *http.Request, rctx rcontext.RequestContext, user UserInfo) interface{} {
	r.WithContext(rctx)
	return next(r, rctx, user)
}

func AccessTokenRequiredRoute(next func(r *http.Request, rctx rcontext.RequestContext, user UserInfo) interface{}) func(*http.Request, rcontext.RequestContext) interface{} {
	return func(r *http.Request, rctx rcontext.RequestContext) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		if accessToken == "" {
			rctx.Log.Error("Error: no token provided (required)")
			return &ErrorResponse{common.ErrCodeMissingToken, "no token provided (required)", common.ErrCodeUnknownToken}
		}
		if config.Get().SharedSecret.Enabled && accessToken == config.Get().SharedSecret.Token {
			log := rctx.Log.WithFields(logrus.Fields{"isRepoAdmin": true})
			log.Info("User authed using shared secret")
			return callUserNext(next, r, rctx, UserInfo{UserId: "@sharedsecret", AccessToken: accessToken, IsShared: true})
		}
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := auth_cache.GetUserId(rctx, accessToken, appserviceUserId)
		if err != nil || userId == "" {
			if err == matrix.ErrGuestToken {
				return GuestAuthFailed()
			}
			if err != nil && err != matrix.ErrInvalidToken {
				sentry.CaptureException(err)
				rctx.Log.Error("Error verifying token (fatal): ", err)
				return InternalServerError("Unexpected Error")
			}

			rctx.Log.Warn("Failed to verify token (fatal): ", err)
			return AuthFailed()
		}

		rctx = rctx.LogWithFields(logrus.Fields{"authUserId": userId})
		return callUserNext(next, r, rctx, UserInfo{userId, accessToken, false})
	}
}

func AccessTokenOptionalRoute(next func(r *http.Request, rctx rcontext.RequestContext, user UserInfo) interface{}) func(*http.Request, rcontext.RequestContext) interface{} {
	return func(r *http.Request, rctx rcontext.RequestContext) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		if accessToken == "" {
			return callUserNext(next, r, rctx, UserInfo{"", "", false})
		}
		if config.Get().SharedSecret.Enabled && accessToken == config.Get().SharedSecret.Token {
			rctx = rctx.LogWithFields(logrus.Fields{"isRepoAdmin": true})
			rctx.Log.Info("User authed using shared secret")
			return callUserNext(next, r, rctx, UserInfo{UserId: "@sharedsecret", AccessToken: accessToken, IsShared: true})
		}
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := auth_cache.GetUserId(rctx, accessToken, appserviceUserId)
		if err != nil {
			if err != matrix.ErrInvalidToken {
				rctx.Log.Error("Error verifying token: ", err)
				return InternalServerError("Unexpected Error")
			}

			rctx.Log.Warn("Failed to verify token (non-fatal): ", err)
			userId = ""
		}

		rctx = rctx.LogWithFields(logrus.Fields{"authUserId": userId})
		return callUserNext(next, r, rctx, UserInfo{userId, accessToken, false})
	}
}

func RepoAdminRoute(next func(r *http.Request, rctx rcontext.RequestContext, user UserInfo) interface{}) func(*http.Request, rcontext.RequestContext) interface{} {
	regularFunc := AccessTokenRequiredRoute(func(r *http.Request, rctx rcontext.RequestContext, user UserInfo) interface{} {
		if user.UserId == "" {
			rctx.Log.Warn("Could not identify user for this admin route")
			return AuthFailed()
		}
		if !util.IsGlobalAdmin(user.UserId) {
			rctx.Log.Warn("User " + user.UserId + " is not a repository administrator")
			return AuthFailed()
		}

		rctx = rctx.LogWithFields(logrus.Fields{"isRepoAdmin": true})
		return callUserNext(next, r, rctx, user)
	})

	return func(r *http.Request, rctx rcontext.RequestContext) interface{} {
		if config.Get().SharedSecret.Enabled {
			accessToken := util.GetAccessTokenFromRequest(r)
			if accessToken == config.Get().SharedSecret.Token {
				rctx = rctx.LogWithFields(logrus.Fields{"isRepoAdmin": true})
				rctx.Log.Info("User authed using shared secret")
				return callUserNext(next, r, rctx, UserInfo{UserId: "@sharedsecret", AccessToken: accessToken, IsShared: true})
			}
		}

		return regularFunc(r, rctx)
	}
}

func GetRequestUserAdminStatus(r *http.Request, rctx rcontext.RequestContext, user UserInfo) (bool, bool) {
	isGlobalAdmin := util.IsGlobalAdmin(user.UserId) || user.IsShared
	isLocalAdmin, err := matrix.IsUserAdmin(rctx, r.Host, user.AccessToken, r.RemoteAddr)
	if err != nil {
		sentry.CaptureException(err)
		rctx.Log.Error("Error verifying local admin: " + err.Error())
		return isGlobalAdmin, false
	}

	return isGlobalAdmin, isLocalAdmin
}
