package custom

import (
	"encoding/json"
	"fmt"
	"github.com/getsentry/sentry-go"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/storage"
	"github.com/turt2live/matrix-media-repo/storage/stores"
	"github.com/turt2live/matrix-media-repo/types"
	"github.com/turt2live/matrix-media-repo/util"
)

type MinimalUsageInfo struct {
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
	RawBytes     *MinimalUsageInfo `json:"raw_bytes"`
	RawCounts    *MinimalUsageInfo `json:"raw_counts"`
	UploadedMxcs []string          `json:"uploaded,flow"`
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

func GetDomainUsage(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	serverName := params["serverName"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
	})

	db := storage.GetDatabase().GetMetadataStore(rctx)

	mediaBytes, thumbBytes, err := db.GetByteUsageForServer(serverName)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return api.InternalServerError("Failed to get byte usage for server")
	}

	mediaCount, thumbCount, err := db.GetCountUsageForServer(serverName)
	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return api.InternalServerError("Failed to get count usage for server")
	}

	return &api.DoNotCacheResponse{
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

func GetUserUsage(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	serverName := params["serverName"]
	userIds := r.URL.Query()["user_id"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
	})

	db := storage.GetDatabase().GetMediaStore(rctx)

	var records []*types.Media
	var err error
	if userIds == nil || len(userIds) == 0 {
		records, err = db.GetAllMediaForServer(serverName)
	} else {
		records, err = db.GetAllMediaForServerUsers(serverName, userIds)
	}

	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return api.InternalServerError("Failed to get media records for users")
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

		entry.UploadedMxcs = append(entry.UploadedMxcs, media.MxcUri())
	}

	return &api.DoNotCacheResponse{Payload: parsed}
}

func GetUploadsUsage(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)

	serverName := params["serverName"]
	mxcs := r.URL.Query()["mxc"]

	rctx = rctx.LogWithFields(logrus.Fields{
		"serverName": serverName,
	})

	db := storage.GetDatabase().GetMediaStore(rctx)

	var records []*types.Media
	var err error
	if mxcs == nil || len(mxcs) == 0 {
		records, err = db.GetAllMediaForServer(serverName)
	} else {
		split := make([]string, 0)
		for _, mxc := range mxcs {
			o, i, err := util.SplitMxc(mxc)
			if err != nil {
				rctx.Log.Error(err)
				sentry.CaptureException(err)
				return api.InternalServerError("Error parsing MXC " + mxc)
			}

			if o != serverName {
				return api.BadRequest("MXC URIs must match the requested server")
			}

			split = append(split, i)
		}
		records, err = db.GetAllMediaInIds(serverName, split)
	}

	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return api.InternalServerError("Failed to get media records for users")
	}

	parsed := make(map[string]*MediaUsageEntry)

	for _, media := range records {
		parsed[media.MxcUri()] = &MediaUsageEntry{
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

	return &api.DoNotCacheResponse{Payload: parsed}
}

// GetUsersUsageStats attempts to provide a loose equivalent to this Synapse admin end-point:
// https://matrix-org.github.io/synapse/develop/admin_api/statistics.html#users-media-usage-statistics
func GetUsersUsageStats(r *http.Request, rctx rcontext.RequestContext, user api.UserInfo) interface{} {
	params := mux.Vars(r)
	qs := r.URL.Query()
	var err error

	serverName := params["serverName"]

	isGlobalAdmin, isLocalAdmin := api.GetRequestUserAdminStatus(r, rctx, user)
	if !isGlobalAdmin && (serverName != r.Host || !isLocalAdmin) {
		return api.AuthFailed()
	}

	orderBy := qs.Get("order_by")
	if len(qs["order_by"]) == 0 {
		orderBy = "user_id"
	}
	if !util.ArrayContains(stores.UsersUsageStatsSorts, orderBy) {
		acceptedValsStr, _ := json.Marshal(stores.UsersUsageStatsSorts)
		acceptedValsStr = []byte(strings.ReplaceAll(string(acceptedValsStr), "\"", "'"))
		return api.BadRequest(
			fmt.Sprintf("Query parameter 'order_by' must be one of %s", acceptedValsStr))
	}

	var start int64 = 0
	if len(qs["from"]) > 0 {
		start, err = strconv.ParseInt(qs.Get("from"), 10, 64)
		if err != nil || start < 0 {
			return api.BadRequest("Query parameter 'from' must be a non-negative integer")
		}
	}

	var limit int64 = 100
	if len(qs["limit"]) > 0 {
		limit, err = strconv.ParseInt(qs.Get("limit"), 10, 64)
		if err != nil || limit < 0 {
			return api.BadRequest("Query parameter 'limit' must be a non-negative integer")
		}
	}

	const unspecifiedTS int64 = -1
	fromTS := unspecifiedTS
	if len(qs["from_ts"]) > 0 {
		fromTS, err = strconv.ParseInt(qs.Get("from_ts"), 10, 64)
		if err != nil || fromTS < 0 {
			return api.BadRequest("Query parameter 'from_ts' must be a non-negative integer")
		}
	}

	untilTS := unspecifiedTS
	if len(qs["until_ts"]) > 0 {
		untilTS, err = strconv.ParseInt(qs.Get("until_ts"), 10, 64)
		if err != nil || untilTS < 0 {
			return api.BadRequest("Query parameter 'until_ts' must be a non-negative integer")
		} else if untilTS <= fromTS {
			return api.BadRequest("Query parameter 'until_ts' must be greater than 'from_ts'")
		}
	}

	searchTerm := qs.Get("search_term")
	if searchTerm == "" && len(qs["search_term"]) > 0 {
		return api.BadRequest("Query parameter 'search_term' cannot be an empty string")
	}

	isAscendingOrder := true
	direction := qs.Get("dir")
	if direction == "f" || len(qs["dir"]) == 0 {
		// Use default order
	} else if direction == "b" {
		isAscendingOrder = false
	} else {
		return api.BadRequest("Query parameter 'dir' must be one of ['f', 'b']")
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

	db := storage.GetDatabase().GetMediaStore(rctx)

	stats, totalCount, err := db.GetUsersUsageStatsForServer(
		serverName,
		orderBy,
		start,
		limit,
		fromTS,
		untilTS,
		searchTerm,
		isAscendingOrder)

	if err != nil {
		rctx.Log.Error(err)
		sentry.CaptureException(err)
		return api.InternalServerError("Failed to get users' usage stats on specified server")
	}

	var users []map[string]interface{}
	if len(stats) == 0 {
		users = []map[string]interface{}{}
	} else {
		for _, record := range stats {
			users = append(users, map[string]interface{}{
				"media_count":  record.MediaCount,
				"media_length": record.MediaLength,
				"user_id":      record.UserId,
			})
		}
	}

	var result = map[string]interface{}{
		"users": users,
		"total": totalCount,
	}
	if (start + limit) < totalCount {
		result["next_token"] = start + int64(len(stats))
	}

	return &api.DoNotCacheResponse{Payload: result}
}
