package r0

import (
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/util"
)

type userInfo struct {
	userId      string
	accessToken string
}

func AccessTokenRequiredRoute(next func(r *http.Request, log *logrus.Entry, user userInfo) interface{}) func(*http.Request, *logrus.Entry) interface{} {
	return func(r *http.Request, log *logrus.Entry) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := matrix.GetUserIdFromToken(r.Context(), r.Host, accessToken, appserviceUserId)
		if err != nil || userId == "" {
			log.Error(err)
			if err != nil && err != matrix.ErrNoToken {
				log.Error("Error verifying token: ", err)
				return api.InternalServerError("Unexpected Error")
			}

			log.Warn("Failed to verify token (fatal)")
			return api.AuthFailed()
		}

		log = log.WithFields(logrus.Fields{"authUserId": userId})
		return next(r, log, userInfo{userId, accessToken})
	}
}

func AccessTokenOptionalRoute(next func(r *http.Request, log *logrus.Entry, user userInfo) interface{}) func(*http.Request, *logrus.Entry) interface{} {
	return func(r *http.Request, log *logrus.Entry) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := matrix.GetUserIdFromToken(r.Context(), r.Host, accessToken, appserviceUserId)
		if err != nil {
			if err != matrix.ErrNoToken {
				log.Error("Error verifying token: ", err)
				return api.InternalServerError("Unexpected Error")
			}

			log.Warn("Failed to verify token (non-fatal)")
			userId = ""
		}

		log = log.WithFields(logrus.Fields{"authUserId": userId})
		return next(r, log, userInfo{userId, accessToken})
	}
}

func RepoAdminRoute(next func(r *http.Request, log *logrus.Entry, user userInfo) interface{}) func(*http.Request, *logrus.Entry) interface{} {
	return AccessTokenRequiredRoute(func(r *http.Request, log *logrus.Entry, user userInfo) interface{} {
		if user.userId == "" {
			log.Warn("Could not identify user for this admin route")
			return api.AuthFailed()
		}
		if !util.IsGlobalAdmin(user.userId) {
			log.Warn("User " + user.userId + " is not a repository administrator")
			return api.AuthFailed()
		}

		log = log.WithFields(logrus.Fields{"isRepoAdmin": true})
		return next(r, log, user)
	})
}
