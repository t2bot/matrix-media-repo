package util

import "golang.org/x/crypto/bcrypt"

func HashString(input string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(input), 14)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
