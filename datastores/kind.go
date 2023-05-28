package datastores

type Kind string

const (
	LocalMediaKind  Kind = "local_media"
	RemoteMediaKind Kind = "remote_media"
	ThumbnailsKind  Kind = "thumbnails"
	ArchivesKind    Kind = "archives"
	AllKind         Kind = "all"
)

func HasListedKind(have []string, want Kind) bool {
	for _, k := range have {
		k2 := Kind(k)
		if k2 == want || k2 == AllKind {
			return true
		}
	}
	return false
}
