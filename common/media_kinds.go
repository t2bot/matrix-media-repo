package common

const KindLocalMedia = "local_media"
const KindRemoteMedia = "remote_media"
const KindThumbnails = "thumbnails"
const KindArchives = "archives"
const KindAll = "all"

var AllKinds = []string{KindLocalMedia, KindRemoteMedia, KindThumbnails}

func IsKind(have string, want string) bool {
	return have == want || have == KindAll
}

func HasKind(have []string, want string) bool {
	for _, k := range have {
		if IsKind(k, want) {
			return true
		}
	}
	return false
}
