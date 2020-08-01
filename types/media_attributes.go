package types

type MediaAttributes struct {
	Origin  string
	MediaId string
	Purpose string
}

const PurposeNone = "none"
const PurposePinned = "pinned"

var AllPurposes = []string{PurposeNone, PurposePinned}
