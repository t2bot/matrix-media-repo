package types

type UrlPreview struct {
	Url         string
	SiteName    string
	Type        string
	Description string
	Title       string
	ImageMxc    string
	ImageType   string
	ImageSize   int64
	ImageWidth  int
	ImageHeight int
}

type CachedUrlPreview struct {
	Preview   *UrlPreview
	SearchUrl string
	ErrorCode string
	FetchedTs int64
}
