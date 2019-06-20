package util

func ArrayContains(a []string, v string) bool {
	for _, e := range a {
		if e == v {
			return true
		}
	}

	return false
}
