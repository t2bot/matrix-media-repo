package responses

type RedirectResponse struct {
	ToUrl string
}

func Redirect(url string) *RedirectResponse {
	return &RedirectResponse{ToUrl: url}
}
