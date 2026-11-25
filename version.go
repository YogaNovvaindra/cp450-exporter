package main

import "fmt"

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func versionSummary() string {
	return fmt.Sprintf("cp450-exporter %s (commit=%s, built=%s)", version, commit, buildDate)
}
