package mageutil

import (
	"fmt"
	"os"

	"github.com/magefile/mage/sh"
)

// Internal datastax DNS addresses
// to use for distros (like Alpine)
// that do not query DNS servers in order.
var DatastaxDns = []string{"10.100.6.66", "10.100.6.67"}

func RunDocker(imageName string, volumes []string, dnsAddrs []string, env []string, runArgs []string, execArgs []string) (string, error) {
	args := []string{"run", "--rm"}

	for _, x := range env {
		args = append(args, "--env")
		args = append(args, x)
	}
	for _, x := range dnsAddrs {
		args = append(args, "--dns")
		args = append(args, x)
	}
	for _, x := range volumes {
		args = append(args, "-v")
		args = append(args, x)
	}
	args = append(args, runArgs...)
	args = append(args, imageName)
	args = append(args, execArgs...)
	return sh.Output("docker", args...)
}

func RequireEnv(s string) string {
	val := os.Getenv(s)
	if val == "" {
		msg := fmt.Errorf("%s is a required environment variable\n", s)
		panic(msg)
	}
	return val
}
