package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/suderio/bg3-guard/cmd"
	"github.com/suderio/bg3-guard/internal/recover"
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
