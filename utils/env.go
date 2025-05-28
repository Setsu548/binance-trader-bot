package utils

import (
	"fmt"
	"os"
	"strconv"
)

// GetEnv retrieves the value of the environment variable named by the key.
// If the variable is not present, it returns an error.
func GetEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable %s not set", key)
	}
	return value, nil
}

// GetEnvOrDefault retrieves the value of the environment variable named by the key.
// If the variable is not present, it returns the provided defaultValue.
func GetEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetEnvAsInt retrieves the value of the environment variable named by the key as an integer.
// Returns an error if the variable is not set or cannot be parsed as an integer.
func GetEnvAsInt(key string) (int, error) {
	str, err := GetEnv(key)
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(str)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s is not a valid integer: %w", key, err)
	}
	return val, nil
}

// GetEnvAsFloat retrieves the value of the environment variable named by the key as a float64.
// Returns an error if the variable is not set or cannot be parsed as a float.
func GetEnvAsFloat(key string) (float64, error) {
	str, err := GetEnv(key)
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s is not a valid float: %w", key, err)
	}
	return val, nil
}

// GetEnvAsBool retrieves the value of the environment variable named by the key as a boolean.
// Returns an error if the variable is not set or cannot be parsed as a boolean.
func GetEnvAsBool(key string) (bool, error) {
	str, err := GetEnv(key)
	if err != nil {
		return false, err
	}
	val, err := strconv.ParseBool(str)
	if err != nil {
		return false, fmt.Errorf("environment variable %s is not a valid boolean: %w", key, err)
	}
	return val, nil
}
