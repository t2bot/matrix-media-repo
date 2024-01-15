package datastores

import "errors"

type RedirectError struct {
	error
	RedirectUrl string
}

func redirect(url string) RedirectError {
	return RedirectError{
		error:       errors.New("redirection"),
		RedirectUrl: url,
	}
}
