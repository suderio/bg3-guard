//go:build ignore

package main


import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HtmlUrl     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

type Module struct {
	Path     string  `json:"Path"`
	Version  string  `json:"Version"`
	Indirect bool    `json:"Indirect"`
	Update   *Module `json:"Update"`
	Main     bool    `json:"Main"`
}

func main() {
	configuredLSLib := getLSLibVersion()


	fmt.Println("==================================================")
	fmt.Println("            BG3 Guard Dependency Check            ")
	fmt.Println("==================================================")
	fmt.Println()

	// 1. Check Go Dependencies
	checkGoModules()

	fmt.Println()
	// 2. Check LSLib Divine Toolchain
	checkLSLib(configuredLSLib)

	fmt.Println("==================================================")
}

func checkGoModules() {
	fmt.Println("[1/2] Checking Go Dependencies (go.mod)...")
	fmt.Println("--------------------------------------------------")

	cmd := exec.Command("go", "list", "-u", "-m", "-json", "all")
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error running 'go list': %v\n", err)
		return
	}

	dec := json.NewDecoder(strings.NewReader(string(out)))
	var directUpdates []string
	var directCount int

	for dec.More() {
		var m Module
		if err := dec.Decode(&m); err != nil {
			break
		}
		if m.Main || m.Indirect {
			continue
		}

		directCount++
		if m.Update != nil {
			msg := fmt.Sprintf("  * %-35s %s -> %s (UPDATE AVAILABLE)", m.Path, m.Version, m.Update.Version)
			fmt.Println(msg)
			directUpdates = append(directUpdates, m.Path)
		} else {
			fmt.Printf("  * %-35s %s (up to date)\n", m.Path, m.Version)
		}
	}

	fmt.Println()
	if len(directUpdates) == 0 {
		fmt.Printf("All %d direct Go dependencies are up to date!\n", directCount)
	} else {
		fmt.Printf("Found %d update(s) available for direct Go dependencies.\n", len(directUpdates))
		fmt.Println("Run 'go get -u <dependency>' to upgrade.")
	}
}

func checkLSLib(configuredVersion string) {
	fmt.Println("[2/2] Checking LSLib Divine Toolchain...")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("  Configured in Makefile: %s\n", configuredVersion)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/Norbyte/lslib/releases/latest", nil)
	if err != nil {
		fmt.Printf("  Error creating request: %v\n", err)
		return
	}
	req.Header.Set("User-Agent", "bg3-guard-check-updates")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  Error connecting to GitHub API: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  GitHub API returned HTTP %s (rate limit or network issue)\n", resp.Status)
		return
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		fmt.Printf("  Failed to parse GitHub response: %v\n", err)
		return
	}

	fmt.Printf("  Latest Release on GitHub: %s (%s)\n", rel.TagName, rel.Name)
	fmt.Printf("  Release URL: %s\n", rel.HtmlUrl)
	fmt.Println()

	if strings.TrimPrefix(rel.TagName, "v") == strings.TrimPrefix(configuredVersion, "v") {
		fmt.Printf("LSLib is up to date (%s)!\n", configuredVersion)
	} else {
		fmt.Printf("UPDATE AVAILABLE: LSLib %s is available (current: %s)!\n", rel.TagName, configuredVersion)
		fmt.Printf("Simply update the version in .lslib-version to '%s'.\n", rel.TagName)
	}
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

