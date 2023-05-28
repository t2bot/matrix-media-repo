package common

type Kind string

const KindLocalMedia Kind = "local_media"
const KindRemoteMedia Kind = "remote_media"
const KindThumbnails Kind = "thumbnails"
const KindArchives Kind = "archives"
const KindAll Kind = "all"

func IsKind(have Kind, want Kind) bool {
	return have == want || have == KindAll
}

func HasKind(have []string, want Kind) bool {
	for _, k := range have {
		if IsKind(Kind(k), want) {
			return true
		}
	}
	return false
}
