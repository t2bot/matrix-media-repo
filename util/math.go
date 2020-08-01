package util

func MaxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func MinInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func MinFloat32(a float32, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
