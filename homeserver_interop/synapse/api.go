package synapse

const PrefixAdminApi = "/_synapse/admin"

type SynUserStatRecord struct {
	DisplayName string `json:"displayname"`
	UserId      string `json:"user_id"`
	MediaCount  int64  `json:"media_count"`
	MediaLength int64  `json:"media_length"`
}

type SynUserStatsResponse struct {
	Users     []*SynUserStatRecord `json:"users,flow"`
	NextToken int64                `json:"next_token,omitempty"`
	Total     int64                `json:"total"`
}
