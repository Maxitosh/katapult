package config

import "os"

// EnvOrDefault returns the value of the environment variable named by key,
// or fallback if the variable is empty or unset.
func EnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
