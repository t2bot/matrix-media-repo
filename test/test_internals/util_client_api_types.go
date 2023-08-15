package test_internals

type MatrixErrorResponse struct {
	InjectedStatusCode int
	Code               string `json:"errcode"`
	Message            string `json:"error"` // optional
}

type MatrixUploadResponse struct {
	MxcUri string `json:"content_uri"`
}

type MatrixCreatedMediaResponse struct {
	*MatrixUploadResponse
	ExpiresTs int64 `json:"unused_expires_at"`
}
