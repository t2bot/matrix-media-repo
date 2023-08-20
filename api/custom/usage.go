package custom

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/api/_routers"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/synapse"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/util"
)

type MinimalUsageInfo struct {
	// Used in per-user endpoints where we can't count thumbnails
	Total int64 `json:"total"`
	Media int64 `json:"media"`
}

type UsageInfo struct {
	*MinimalUsageInfo
	Thumbnails int64 `json:"thumbnails"`
}

type CountsUsageResponse struct {
	RawBytes  *UsageInfo `json:"raw_bytes"`
	RawCounts *UsageInfo `json:"raw_counts"`
}

type UserUsageEntry struct {
	// Returned by per-user endpoints, where we can't count thumbnails
	RawBytes     *MinimalUsageInfo `json:"raw_bytes"`
	RawCounts    *MinimalUsageInfo `json:"raw_counts"`
	UploadedMxcs []string          `json:"uploaded"`
}

type MediaUsageEntry struct {
	SizeBytes         int64  `json:"size_bytes"`
	UploadedBy        string `json:"uploaded_by"`
	DatastoreId       string `json:"datastore_id"`
	DatastoreLocation string `json:"datastore_location"`
	Sha256Hash        string `json:"sha256_hash"`
	Quarantined       bool   `json:"quarantined"`
	UploadName        string `json:"upload_name"`
	ContentType       string `json:"content_type"`
	CreatedTs         int64  `json:"created_ts"`
}

func GetDomainUsage(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	serverName := _routers.GetParam("serverName", r)

	if !_routers.ServerNameRegex.MatchString(serverName) {
		return _responses.BadRequest("invalid server name")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
	})

	db := database.GetInstance().MetadataView.Prepare(rctx)

	mediaBytes, thumbBytes, err := db.ByteUsageForServer(serverName)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get byte usage for server")
	}

	mediaCount, thumbCount, err := db.CountUsageForServer(serverName)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get count usage for server")
	}

	return &_responses.DoNotCacheResponse{
		Payload: &CountsUsageResponse{
			RawBytes: &UsageInfo{
				MinimalUsageInfo: &MinimalUsageInfo{
					Total: mediaBytes + thumbBytes,
					Media: mediaBytes,
				},
				Thumbnails: thumbBytes,
			},
			RawCounts: &UsageInfo{
				MinimalUsageInfo: &MinimalUsageInfo{
					Total: mediaCount + thumbCount,
					Media: mediaCount,
				},
				Thumbnails: thumbCount,
			},
		},
	}
}

func GetUserUsage(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	serverName := _routers.GetParam("serverName", r)
	userIds := r.URL.Query()["user_id"]

	if !_routers.ServerNameRegex.MatchString(serverName) {
		return _responses.BadRequest("invalid server name")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
	})

	db := database.GetInstance().Media.Prepare(rctx)

	var records []*database.DbMedia
	var err error
	if len(userIds) == 0 {
		records, err = db.GetByOrigin(serverName)
	} else {
		records, err = db.GetByOriginUsers(serverName, userIds)
	}

	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get media records for users")
	}

	parsed := make(map[string]*UserUsageEntry)

	for _, media := range records {
		var entry *UserUsageEntry
		var ok bool
		if entry, ok = parsed[media.UserId]; !ok {
			entry = &UserUsageEntry{
				UploadedMxcs: make([]string, 0),
				RawCounts: &MinimalUsageInfo{
					Total: 0,
					Media: 0,
				},
				RawBytes: &MinimalUsageInfo{
					Total: 0,
					Media: 0,
				},
			}
			parsed[media.UserId] = entry
		}

		entry.RawBytes.Total += media.SizeBytes
		entry.RawBytes.Media += media.SizeBytes
		entry.RawCounts.Total += 1
		entry.RawCounts.Media += 1

		entry.UploadedMxcs = append(entry.UploadedMxcs, util.MxcUri(media.Origin, media.MediaId))
	}

	return &_responses.DoNotCacheResponse{Payload: parsed}
}

func GetUploadsUsage(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	serverName := _routers.GetParam("serverName", r)
	mxcs := r.URL.Query()["mxc"]

	if !_routers.ServerNameRegex.MatchString(serverName) {
		return _responses.BadRequest("invalid server name")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
	})

	db := database.GetInstance().Media.Prepare(rctx)

	var records []*database.DbMedia
	var err error
	if len(mxcs) == 0 {
		records, err = db.GetByOrigin(serverName)
	} else {
		split := make([]string, 0)
		for _, mxc := range mxcs {
			o, i, err := util.SplitMxc(mxc)
			if err != nil {
				rctx.Log.Error(err)
				sentry.CaptureException(err)
				return _responses.InternalServerError("Error parsing MXC " + mxc)
			}

			if o != serverName {
				return _responses.BadRequest("MXC URIs must match the requested server")
			}

			split = append(split, i)
		}
		records, err = db.GetByIds(serverName, split)
	}

	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get media records for users")
	}

	parsed := make(map[string]*MediaUsageEntry)

	for _, media := range records {
		parsed[util.MxcUri(media.Origin, media.MediaId)] = &MediaUsageEntry{
			SizeBytes:         media.SizeBytes,
			UploadName:        media.UploadName,
			ContentType:       media.ContentType,
			CreatedTs:         media.CreationTs,
			DatastoreId:       media.DatastoreId,
			DatastoreLocation: media.Location,
			Quarantined:       media.Quarantined,
			Sha256Hash:        media.Sha256Hash,
			UploadedBy:        media.UserId,
		}
	}

	return &_responses.DoNotCacheResponse{Payload: parsed}
}

// SynGetUsersMediaStats attempts to provide a loose equivalent to this Synapse admin endpoint:
// https://matrix-org.github.io/synapse/v1.88/admin_api/statistics.html#users-media-usage-statistics
func SynGetUsersMediaStats(r *http.Request, rctx rcontext.RequestContext, user _apimeta.UserInfo) interface{} {
	qs := r.URL.Query()
	var err error

	serverName := _routers.GetParam("serverName", r)
	if serverName == "" && strings.HasPrefix(r.URL.Path, synapse.PrefixAdminApi) {
		serverName = r.Host
	}

	if !_routers.ServerNameRegex.MatchString(serverName) {
		return _responses.BadRequest("invalid server name")
	}

	isGlobalAdmin, isLocalAdmin := _apimeta.GetRequestUserAdminStatus(r, rctx, user)
	if !isGlobalAdmin && (serverName != r.Host || !isLocalAdmin) {
		return _responses.AuthFailed()
	}

	orderBy := database.SynStatUserOrderBy(qs.Get("order_by"))
	if len(qs["order_by"]) == 0 {
		orderBy = database.DefaultSynStatUserOrderBy
	}
	if !database.IsSynStatUserOrderBy(orderBy) {
		return _responses.BadRequest("Query parameter 'order_by' must be one of the accepted values")
	}

	var start int64 = 0
	if len(qs["from"]) > 0 {
		start, err = strconv.ParseInt(qs.Get("from"), 10, 64)
		if err != nil || start < 0 {
			return _responses.BadRequest("Query parameter 'from' must be a non-negative integer")
		}
	}

	var limit int64 = 100
	if len(qs["limit"]) > 0 {
		limit, err = strconv.ParseInt(qs.Get("limit"), 10, 64)
		if err != nil || limit < 0 {
			return _responses.BadRequest("Query parameter 'limit' must be a non-negative integer")
		}
	}
	if limit > 50 {
		limit = 50
	}

	const unspecifiedTS int64 = -1
	fromTS := unspecifiedTS
	if len(qs["from_ts"]) > 0 {
		fromTS, err = strconv.ParseInt(qs.Get("from_ts"), 10, 64)
		if err != nil || fromTS < 0 {
			return _responses.BadRequest("Query parameter 'from_ts' must be a non-negative integer")
		}
	}

	untilTS := unspecifiedTS
	if len(qs["until_ts"]) > 0 {
		untilTS, err = strconv.ParseInt(qs.Get("until_ts"), 10, 64)
		if err != nil || untilTS < 0 {
			return _responses.BadRequest("Query parameter 'until_ts' must be a non-negative integer")
		} else if untilTS <= fromTS {
			return _responses.BadRequest("Query parameter 'until_ts' must be greater than 'from_ts'")
		}
	}

	searchTerm := qs.Get("search_term")
	if searchTerm == "" && len(qs["search_term"]) > 0 {
		return _responses.BadRequest("Query parameter 'search_term' cannot be an empty string")
	}

	isAscendingOrder := true
	direction := qs.Get("dir")
	if direction == "b" && len(qs["dir"]) > 0 {
		isAscendingOrder = false
	} else {
		return _responses.BadRequest("Query parameter 'dir' must be one of ['f', 'b']")
	}

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName":       serverName,
		"order_by":         orderBy,
		"from":             start,
		"limit":            limit,
		"from_ts":          fromTS,
		"until_ts":         untilTS,
		"search_term":      searchTerm,
		"isAscendingOrder": isAscendingOrder,
	})

	db := database.GetInstance().MetadataView.Prepare(rctx)

	stats, totalCount, err := db.UnoptimizedSynapseUserStatsPage(serverName, orderBy, start, limit, fromTS, untilTS, searchTerm, isAscendingOrder)

	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return _responses.InternalServerError("Failed to get users' usage stats on specified server")
	}

	result := &synapse.SynUserStatsResponse{
		Users:     make([]*synapse.SynUserStatRecord, 0),
		NextToken: 0, // invoke omitEmpty by default
		Total:     totalCount,
	}
	for _, record := range stats {
		result.Users = append(result.Users, &synapse.SynUserStatRecord{
			DisplayName: record.UserId, // TODO: try to populate?
			UserId:      record.UserId,
			MediaCount:  record.MediaCount,
			MediaLength: record.MediaLength,
		})
	}

	if (start + limit) < totalCount {
		result.NextToken = start + int64(len(stats))
	}

	return &_responses.DoNotCacheResponse{Payload: result}
}
