package u

func AdjustProperties(srcWidth int, srcHeight int, desiredWidth int, desiredHeight int, wantAnimated bool, method string) (bool, int, int, bool, string) {
	aspectRatio := float32(srcHeight) / float32(srcWidth)
	targetAspectRatio := float32(desiredHeight) / float32(desiredWidth)
	if aspectRatio == targetAspectRatio {
		// Super unlikely, but adjust to scale anyways
		method = "scale"
	}

	if srcWidth <= desiredWidth && srcHeight <= desiredHeight {
		if wantAnimated {
			return true, srcWidth, srcHeight, true, method
		} else {
			return false, desiredWidth, desiredHeight, false, method
		}
	}
	return true, desiredWidth, desiredHeight, wantAnimated, method
}
