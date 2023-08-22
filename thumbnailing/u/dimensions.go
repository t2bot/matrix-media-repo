package u

func AdjustProperties(srcWidth int, srcHeight int, desiredWidth int, desiredHeight int, wantAnimated bool, canAnimate bool, method string) (bool, int, int, bool, string) {
	//srcWidth := img.Bounds().Max.X
	//srcHeight := img.Bounds().Max.Y

	aspectRatio := float32(srcHeight) / float32(srcWidth)
	targetAspectRatio := float32(desiredHeight) / float32(desiredWidth)
	if aspectRatio == targetAspectRatio {
		// Super unlikely, but adjust to scale anyways
		method = "scale"
	}

	if srcWidth <= desiredWidth && srcHeight <= desiredHeight {
		if wantAnimated {
			return true, srcWidth, srcHeight, true, method
		} else if canAnimate {
			return true, srcWidth, srcHeight, false, method
		} else {
			return false, 0, 0, false, method
		}
	}
	return true, desiredWidth, desiredHeight, wantAnimated, method
}
