package common

type MmrContextKey string

const (
	ContextLogger           MmrContextKey = "mmr.logger"
	ContextIgnoreHost       MmrContextKey = "mmr.ignore_host"
	ContextAction           MmrContextKey = "mmr.action"
	ContextRequest          MmrContextKey = "mmr.request"
	ContextRequestId        MmrContextKey = "mmr.request_id"
	ContextRequestStartTime MmrContextKey = "mmr.request_start_time"
	ContextServerConfig     MmrContextKey = "mmr.serverConfig"
	ContextDomainConfig     MmrContextKey = "mmr.domain_config"
	ContextStatusCode       MmrContextKey = "mmr.status_code"
)
