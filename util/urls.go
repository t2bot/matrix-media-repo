package util

func MakeUrl(parts ...string) string {
	res := ""
	for i, p := range parts {
		if p[len(p)-1:] == "/" {
			res += p[:len(p)-1]
		} else if p[0] != '/' && i > 0 {
			res += "/" + p
		} else {
			res += p
		}
	}
	return res
}