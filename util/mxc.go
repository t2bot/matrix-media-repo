package util

func MxcUri(origin string, mediaId string) string {
	return "mxc://" + origin + "/" + mediaId
}
