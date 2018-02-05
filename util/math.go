package util

// int64 min function
func Min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

// int64 max function
func Max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}
