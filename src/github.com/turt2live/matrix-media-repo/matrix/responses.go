package matrix

type userIdResponse struct {
	UserId string `json:"user_id"`
}

type whoisResponse struct {
	// We don't actually care about any of the fields here
}

type mediaListResponse struct {
	LocalMxcs  []string `json:"local"`
	RemoteMxcs []string `json:"remote"`
}
