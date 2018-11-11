package api

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/util"
)

type UserInfo struct {
	UserId      string
	AccessToken string
}

func AccessTokenRequiredRoute(next func(r *http.Request, log *logrus.Entry, user UserInfo) interface{}) func(*http.Request, *logrus.Entry) interface{} {
	return func(r *http.Request, log *logrus.Entry) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := matrix.GetUserIdFromToken(r.Context(), r.Host, accessToken, appserviceUserId, r.RemoteAddr)
		if err != nil || userId == "" {
			if err != nil && err != matrix.ErrNoToken {
				log.Error("Error verifying token: ", err)
				return InternalServerError("Unexpected Error")
			}

			log.Warn("Failed to verify token (fatal)")
			return AuthFailed()
		}

		log = log.WithFields(logrus.Fields{"authUserId": userId})
		return next(r, log, UserInfo{userId, accessToken})
	}
}

func AccessTokenOptionalRoute(next func(r *http.Request, log *logrus.Entry, user UserInfo) interface{}) func(*http.Request, *logrus.Entry) interface{} {
	return func(r *http.Request, log *logrus.Entry) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := matrix.GetUserIdFromToken(r.Context(), r.Host, accessToken, appserviceUserId, r.RemoteAddr)
		if err != nil {
			if err != matrix.ErrNoToken {
				log.Error("Error verifying token: ", err)
				return InternalServerError("Unexpected Error")
			}

			log.Warn("Failed to verify token (non-fatal)")
			userId = ""
		}

		log = log.WithFields(logrus.Fields{"authUserId": userId})
		return next(r, log, UserInfo{userId, accessToken})
	}
}

func RepoAdminRoute(next func(r *http.Request, log *logrus.Entry, user UserInfo) interface{}) func(*http.Request, *logrus.Entry) interface{} {
	return AccessTokenRequiredRoute(func(r *http.Request, log *logrus.Entry, user UserInfo) interface{} {
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
}
