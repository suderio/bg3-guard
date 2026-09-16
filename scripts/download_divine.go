//go:build ignore

package main


import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	version := getLSLibVersion()

	destDir := filepath.Join("assets", "divine")
	targetExe1 := filepath.Join(destDir, "Divine.exe")
	targetExe2 := filepath.Join(destDir, "divine.exe")

	if fileExists(targetExe1) || fileExists(targetExe2) {
		fmt.Println("Divine toolchain already present in", destDir)
		return
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", destDir, err)
		os.Exit(1)
	}

	url := fmt.Sprintf("https://github.com/Norbyte/lslib/releases/download/%s/ExportTool-%s.zip", version, version)
	fmt.Printf("Downloading LSLib %s from %s...\n", version, url)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading LSLib: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Download failed with HTTP status: %s\n", resp.Status)
		os.Exit(1)
	}

	tmpZip, err := os.CreateTemp("", "lslib_*.zip")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tmpZip.Name())
	defer tmpZip.Close()

	if _, err := io.Copy(tmpZip, resp.Body); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving zip: %v\n", err)
		os.Exit(1)
	}

	zipReader, err := zip.OpenReader(tmpZip.Name())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening zip: %v\n", err)
		os.Exit(1)
	}
	defer zipReader.Close()

	prefix := "Packed/Tools/"
	extracted := 0
	for _, file := range zipReader.File {
		cleanName := filepath.ToSlash(file.Name)
		if !strings.HasPrefix(cleanName, prefix) {
			continue
		}
		relPath := strings.TrimPrefix(cleanName, prefix)
		if relPath == "" || file.FileInfo().IsDir() {
			continue
		}

		outPath := filepath.Join(destDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating dir for %s: %v\n", outPath, err)
			os.Exit(1)
		}

		rc, err := file.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening zip file entry %s: %v\n", file.Name, err)
			os.Exit(1)
		}

		outFile, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", outPath, err)
			os.Exit(1)
		}

		_, copyErr := io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if copyErr != nil {
			fmt.Fprintf(os.Stderr, "Error extracting %s: %v\n", outPath, copyErr)
			os.Exit(1)
		}
		extracted++
	}

	fmt.Printf("LSLib tools extracted (%d files) to %s\n", extracted, destDir)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func getLSLibVersion() string {
	if len(os.Args) > 1 && os.Args[1] != "" {
		return os.Args[1]
	}
	if data, err := os.ReadFile(".lslib-version"); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			return v
		}
	}
	return "v1.20.4"
}

