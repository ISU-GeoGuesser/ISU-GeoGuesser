package utils

import (
	"iter"
	"log"
	"os"
)

func GetEnvFatal(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalf("missing required environment variable: %s", key)
	}

	return val
}

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
