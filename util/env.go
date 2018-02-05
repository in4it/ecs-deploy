package util

import (
	"os"
)

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func EnvExists(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}
