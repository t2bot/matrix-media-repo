package r0

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/client"
	"github.com/turt2live/matrix-media-repo/rcontext"
	"github.com/turt2live/matrix-media-repo/services"
	"github.com/turt2live/matrix-media-repo/util"
)

type DownloadMediaResponse struct {
	ContentType string
	Filename    string
	SizeBytes   int64
	Location    string
}

func DownloadMedia(w http.ResponseWriter, r *http.Request, i rcontext.RequestInfo) interface{} {
	if !ValidateUserCanDownload(r, i) {
		return client.AuthFailed()
	}

	params := mux.Vars(r)

	server := params["server"]
	mediaId := params["mediaId"]
	filename := params["filename"]

	i.Log = i.Log.WithFields(logrus.Fields{
		"mediaId":  mediaId,
		"server":   server,
		"filename": filename,
	})

	svc := services.CreateMediaService(i)

	media, err := svc.GetMedia(server, mediaId)
	if err != nil {
		if err == util.ErrMediaNotFound {
			return client.NotFoundError()
		} else if err == util.ErrMediaTooLarge {
			return client.RequestTooLarge()
		}
		i.Log.Error("Unexpected error locating media: " + err.Error())
		return client.InternalServerError("Unexpected Error")
	}

	if filename == "" {
		filename = media.UploadName
	}

	return &DownloadMediaResponse{
		ContentType: media.ContentType,
		Filename:    filename,
		SizeBytes:   media.SizeBytes,
		Location:    media.Location,
	}
}

func ValidateUserCanDownload(r *http.Request, i rcontext.RequestInfo) (bool) {
	hs := util.GetHomeserverConfig(r.Host, i.Config)
	if !hs.DownloadRequiresAuth {
		return true // no auth required == can access
	}

	accessToken := util.GetAccessTokenFromRequest(r)
	userId, err := util.GetUserIdFromToken(i.Context, r.Host, accessToken, i.Config)
	return userId != "" && err != nil
}
