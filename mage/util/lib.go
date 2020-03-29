// Copyright DataStax, Inc.
// Please see the included license file for details.

package mageutil

import (
	"fmt"
	"math/rand"
	"os"
)

// Creates and sets permissions on a directory. Idempotent.
func EnsureDir(dir string) {
	os.Mkdir(dir, os.ModePerm)
	//For some reason, this step is necessary to actually
	//get the expected permissions
	os.Chmod(dir, os.ModePerm)
}

func RequireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		msg := fmt.Errorf("%s is a required environment variable\n", key)
		panic(msg)
	}
	return val
}

// Attempt to retrieve a value from an environment
// variable if it is set. If not, use specified
// default value
func FromEnvOrDefault(key string, def string) string {
	var val string
	if v, ok := os.LookupEnv(key); ok {
		val = v
	} else {
		val = def
	}
	return val
}

// Attempt to retrieve a value from an environment
// variable if it is set. If not, use fallback
// function to generate value.
func FromEnvOrF(key string, fallback func() string) string {
	var val string
	if v, ok := os.LookupEnv(key); ok {
		val = v
	} else {
		val = fallback()
	}
	return val
}

func EnvOrDefault(key string, def string) string {
	val := os.Getenv(key)
	if val == "" {
		val = def
	}
	return val
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

// Generates a (non-cryptographically) random hex string of a given length
func RandomHex(length int) string {
	hexRunes := []rune("0123456789ABCDEF")
	randRunes := make([]rune, length)
	for i := range randRunes {
		hexIndex := rand.Intn(len(hexRunes))
		randRunes[i] = hexRunes[hexIndex]
	}
	return string(randRunes)
}
