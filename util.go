package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	globHttpClient *http.Client
)

// parseRepoName returns owner and repository name from full repository name
func parseRepoName(fullRepoName string) (string, string, error) {
	parts := strings.SplitN(fullRepoName, "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("cannot parse repository name: %s (owner/repo expected)", fullRepoName)
	}

	return parts[0], parts[1], nil
}

func initHttpClient() {
	globHttpClient = &http.Client{Timeout: 15 * time.Second}
}
