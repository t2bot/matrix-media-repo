package util

func IsAnimatedPNG(b []byte) bool {
	IDAT := []byte{0x49, 0x44, 0x41, 0x54}
	acTL := []byte{0x61, 0x63, 0x54, 0x4C}

	idatIdx := 0
	actlIdx := 0
	for _, bt := range b {
		if bt == IDAT[idatIdx] {
			idatIdx++
			actlIdx = 0
		} else if bt == acTL[actlIdx] {
			actlIdx++
			idatIdx = 0
		} else {
			idatIdx = 0
			actlIdx = 0
		}

		if idatIdx == len(IDAT) {
			return false
		}
		if actlIdx == len(acTL) {
			return true
		}
	}

	return false
}

func IsAnimatedWebp(b []byte) bool {
	// https://stackoverflow.com/a/61242086
	// first we validate the header
	header := []byte("VP8X")
	i := 0
	for i < 4 {
		if b[12 + i] != header[i] {
			return false
		}
		i++
	}
	// now we validate the flag
	flagByte := b[20];
	return ((flagByte >> 1) & 1) > 0
}
