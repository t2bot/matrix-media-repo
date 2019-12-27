package api

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/util"
)

type UserInfo struct {
	UserId      string
	AccessToken string
	IsShared    bool
}

func AccessTokenRequiredRoute(next func(r *http.Request, log *logrus.Entry, user UserInfo) interface{}) func(*http.Request, *logrus.Entry) interface{} {
	return func(r *http.Request, log *logrus.Entry) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		if accessToken == "" {
			log.Error("Error: no token provided (required)")
			return &ErrorResponse{common.ErrCodeMissingToken, "no token provided (required)", common.ErrCodeUnknownToken}
		}
		if config.Get().SharedSecret.Enabled && accessToken == config.Get().SharedSecret.Token {
			log = log.WithFields(logrus.Fields{"isRepoAdmin": true})
			log.Info("User authed using shared secret")
			return next(r, log, UserInfo{UserId: "@sharedsecret", AccessToken: accessToken, IsShared: true})
		}
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := matrix.GetUserIdFromToken(r.Context(), r.Host, accessToken, appserviceUserId, r.RemoteAddr)
		if err != nil || userId == "" {
			if err != nil && err != matrix.ErrNoToken {
				log.Error("Error verifying token: ", err)
				return InternalServerError("Unexpected Error")
			}

			log.Warn("Failed to verify token (fatal): ", err)
			return AuthFailed()
		}

		log = log.WithFields(logrus.Fields{"authUserId": userId})
		return next(r, log, UserInfo{userId, accessToken, false})
	}
}

func AccessTokenOptionalRoute(next func(r *http.Request, log *logrus.Entry, user UserInfo) interface{}) func(*http.Request, *logrus.Entry) interface{} {
	return func(r *http.Request, log *logrus.Entry) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		if accessToken == "" {
			return next(r, log, UserInfo{"", "", false})
		}
		if config.Get().SharedSecret.Enabled && accessToken == config.Get().SharedSecret.Token {
			log = log.WithFields(logrus.Fields{"isRepoAdmin": true})
			log.Info("User authed using shared secret")
			return next(r, log, UserInfo{UserId: "@sharedsecret", AccessToken: accessToken, IsShared: true})
		}
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := matrix.GetUserIdFromToken(r.Context(), r.Host, accessToken, appserviceUserId, r.RemoteAddr)
		if err != nil {
			if err != matrix.ErrNoToken {
				log.Error("Error verifying token: ", err)
				return InternalServerError("Unexpected Error")
			}

			log.Warn("Failed to verify token (non-fatal): ", err)
			userId = ""
		}

		log = log.WithFields(logrus.Fields{"authUserId": userId})
		return next(r, log, UserInfo{userId, accessToken, false})
	}
}

func RepoAdminRoute(next func(r *http.Request, log *logrus.Entry, user UserInfo) interface{}) func(*http.Request, *logrus.Entry) interface{} {
	regularFunc := AccessTokenRequiredRoute(func(r *http.Request, log *logrus.Entry, user UserInfo) interface{} {
		if user.UserId == "" {
			log.Warn("Could not identify user for this admin route")
			return AuthFailed()
		}
		if !util.IsGlobalAdmin(user.UserId) {
			log.Warn("User " + user.UserId + " is not a repository administrator")
			return AuthFailed()
		}

		log = log.WithFields(logrus.Fields{"isRepoAdmin": true})
		return next(r, log, user)
	})

	return func(r *http.Request, log *logrus.Entry) interface{} {
		if config.Get().SharedSecret.Enabled {
			accessToken := util.GetAccessTokenFromRequest(r)
			if accessToken == config.Get().SharedSecret.Token {
				log = log.WithFields(logrus.Fields{"isRepoAdmin": true})
				log.Info("User authed using shared secret")
				return next(r, log, UserInfo{UserId: "@sharedsecret", AccessToken: accessToken, IsShared: true})
			}
		}

		return regularFunc(r, log)
	}
}
