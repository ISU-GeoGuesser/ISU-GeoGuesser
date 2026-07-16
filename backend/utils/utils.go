package utils

import "iter"

func All[T any](seq iter.Seq[T], pred func(T) bool) bool {
	for val := range seq {
		if !pred(val) {
			return false
		}
	}

	return true
}

func Squaref(v float64) float64 {
	return v * v
}
