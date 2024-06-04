package limits

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/libstring"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/t2bot/matrix-media-repo/api/_responses"
	"github.com/t2bot/matrix-media-repo/common/config"
)

var requestLimiter *limiter.Limiter

func init() {
	requestLimiter = tollbooth.NewLimiter(0, nil)
	requestLimiter.SetIPLookups([]string{"X-Forwarded-For", "X-Real-IP", "RemoteAddr"})
	requestLimiter.SetTokenBucketExpirationTTL(time.Hour)

	b, _ := json.Marshal(_responses.RateLimitReached())
	requestLimiter.SetMessage(string(b))
	requestLimiter.SetMessageContentType("application/json")
}

func GetRequestLimiter() *limiter.Limiter {
	requestLimiter.SetBurst(config.Get().RateLimit.BurstCount)
	requestLimiter.SetMax(config.Get().RateLimit.RequestsPerSecond)

	return requestLimiter
}

func GetRequestIP(r *http.Request) string {
	// Same implementation as tollbooth
	return libstring.RemoteIP(requestLimiter.GetIPLookups(), requestLimiter.GetForwardedForIndexFromBehind(), r)
}
