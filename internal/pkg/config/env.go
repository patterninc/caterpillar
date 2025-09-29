package config

import (
	"os"
)

func getEnvironmentVariable(key string) (string, error) {
	return os.Getenv(key), nil
}
