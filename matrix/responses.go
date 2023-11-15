package matrix

type emptyResponse struct {
}

type userIdResponse struct {
	UserId   string `json:"user_id"`
	IsGuest  bool   `json:"org.matrix.msc3069.is_guest"`
	IsGuest2 bool   `json:"is_guest"`
}

type whoisResponse struct {
	// We don't actually care about any of the fields here
}

type MediaListResponse struct {
	LocalMxcs  []string `json:"local"`
	RemoteMxcs []string `json:"remote"`
}

type wellknownServerResponse struct {
	ServerAddr string `json:"m.server"`
}
