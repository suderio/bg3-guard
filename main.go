package main

import (
	"embed"
	"fmt"
	"os"

	"bg3-guard/cmd"
	"bg3-guard/internal/recover"
)

//go:embed assets/divine/*
var divineAssets embed.FS

func init() {
	recover.EmbeddedAssets = divineAssets
	recover.HasEmbeddedAssets = true
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
