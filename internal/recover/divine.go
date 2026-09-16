package recover

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// EmbeddedAssets holds the embedded divine toolchain if bundled during build.
// Can be set from main package or embed directive.
var EmbeddedAssets embed.FS
var HasEmbeddedAssets bool

// GetDivineDir returns the directory where divine tools are cached.
func GetDivineDir() (string, error) {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "bg3-guard", "bin"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "bg3-guard", "bin"), nil
}

// EnsureDivineExecutable locates divine.exe or extracts it from assets.
func EnsureDivineExecutable() (string, error) {
	// 1. Check relative assets/divine folder
	localCandidates := []string{
		filepath.Join(".", "assets", "divine", "Divine.exe"),
		filepath.Join(".", "assets", "divine", "divine.exe"),
		filepath.Join(".", "Divine.exe"),
		filepath.Join(".", "divine.exe"),
	}
	for _, c := range localCandidates {
		if abs, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs, nil
			}
		}
	}

	// 2. Check system PATH
	if path, err := exec.LookPath("divine.exe"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("divine"); err == nil {
		return path, nil
	}

	// 3. Check bg3-guard cache bin dir
	cacheDir, err := GetDivineDir()
	if err == nil {
		cachedExe := filepath.Join(cacheDir, "Divine.exe")
		if _, err := os.Stat(cachedExe); err == nil {
			return cachedExe, nil
		}
	}

	// 4. If embedded assets available, extract them to cacheDir
	if HasEmbeddedAssets && cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create divine cache dir: %w", err)
		}
		err := fs.WalkDir(EmbeddedAssets, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := EmbeddedAssets.ReadFile(path)
			if err != nil {
				return err
			}
			rel := strings.TrimPrefix(path, "assets/divine/")
			rel = strings.TrimPrefix(rel, "assets/divine")
			targetPath := filepath.Join(cacheDir, rel)
			_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
			return os.WriteFile(targetPath, data, 0755)
		})
		if err == nil {
			cachedExe := filepath.Join(cacheDir, "Divine.exe")
			if _, err := os.Stat(cachedExe); err == nil {
				return cachedExe, nil
			}
		}
	}

	return "", fmt.Errorf("divine.exe not found. Run 'make download-divine' or place divine.exe in assets/divine/ or PATH")
}

// RunDivine runs a divine command with the specified arguments.
func RunDivine(args ...string) (string, error) {
	divineExe, err := EnsureDivineExecutable()
	if err != nil {
		return "", err
	}

	cmdArgs := append([]string{"-g", "bg3"}, args...)
	cmd := exec.Command(divineExe, cmdArgs...)

	// Ensure execution directory is the directory containing divine.exe so it finds its companion DLLs
	cmd.Dir = filepath.Dir(divineExe)

	out, err := cmd.CombinedOutput()
	outputStr := string(out)
	if err != nil {
		return outputStr, fmt.Errorf("divine failed (%w): %s", err, outputStr)
	}
	return outputStr, nil
}

// ExtractPackage extracts an .lsv package into destDir.
func ExtractPackage(lsvPath, destDir string) error {
	_, err := RunDivine("-a", "extract-package", "-s", lsvPath, "-d", destDir)
	return err
}

// CreatePackage creates a package (.lsv / .pak) from sourceDir using LZ4HC compression.
func CreatePackage(sourceDir, destLsvPath string) error {
	_, err := RunDivine("-a", "create-package", "-s", sourceDir, "-d", destLsvPath, "-c", "lz4hc")
	return err
}

// ConvertResource converts between .lsf and .lsx (XML).
func ConvertResource(sourcePath, destPath string) error {
	_, err := RunDivine("-a", "convert-resource", "-s", sourcePath, "-d", destPath)
	return err
}
