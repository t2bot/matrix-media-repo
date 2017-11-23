package types

import "github.com/turt2live/matrix-media-repo/client/r0"

type UrlPreview struct {
	Url string
	SiteName string
	Type string
	Description string
	Title string
	ImageMxc string
	ImageType string
	ImageSize int64
	ImageWidth int
	ImageHeight int
}

func (p *UrlPreview) ToOpenGraphResponse() r0.MatrixOpenGraph {
	return r0.MatrixOpenGraph{
		Url: p.Url,
		SiteName: p.SiteName,
		Type: p.Type,
		Description: p.Description,
		Title: p.Title,
		ImageMxc: p.ImageMxc,
		ImageType: p.ImageType,
		ImageSize: p.ImageSize,
		ImageWidth: p.ImageWidth,
		ImageHeight: p.ImageHeight,
	}
}