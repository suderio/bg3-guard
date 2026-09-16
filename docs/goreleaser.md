# Walkthrough - GoReleaser & Multi-Channel Distribution Automation

The Go application **BG3 Guard** (`bg3-guard`) has been configured with **GoReleaser v2** and **GitHub Actions** to automate the entire release lifecycle:

- Cross-platform compilation (Windows, Linux, macOS for amd64 & arm64)
- Automated semantic version injection via linker flags
- Checksum generation (`checksums.txt`)
- Cryptographic keyless signing with **Cosign** (Sigstore OIDC)
- Multi-package manager distribution (**Homebrew**, **Scoop**, **Winget**, **AUR**, **DEB**, and **RPM**)
- Automated publication to **GitHub Releases**
- Multi-channel broadcasting to social media (**Bluesky**, **Discord**, **Mastodon**, **Twitter/X**, **Telegram**, **Reddit**, and **Webhooks**)
- **MIT License** integration

---

## Files Configured & Created

| File | Status | Description |
| :--- | :--- | :--- |
| [LICENSE](../LICENSE) | **Created** | Standard MIT License attributed to `suderio`. |
| [.goreleaser.yaml](../.goreleaser.yaml) | **Created** | Complete GoReleaser v2 specification for builds, signing, packaging, releases, and announcers. |
| [.github/workflows/release.yml](../.github/workflows/release.yml) | **Created** | GitHub Actions workflow executing on tag push `v*` and `workflow_dispatch`. |
| [docs/release-guide.md](../docs/release-guide.md) | **Created** | Step-by-step instructions for repository setup, GitHub Secrets, and release triggers. |
| [cmd/root.go](../cmd/root.go) | **Modified** | Added `Version`, `Commit`, `Date`, `BuiltBy` variables, `--version` template, and `version` subcommand. |
| [.gitignore](../.gitignore) | **Modified** | Added `dist/`, `.local/`, and `config.yaml` to prevent committing build artifacts. |
| [README.md](../README.md) | **Modified** | Added package installation instructions (Scoop, Homebrew, AUR, Releases) and License section. |

---

## Key Features Implemented

### 1. Cross-Platform Compilation & Versioning

- GoReleaser compiles standalone binaries for:
  - **Windows**: `amd64`, `arm64` (packaged as `.zip`)
  - **Linux**: `amd64`, `arm64` (packaged as `.tar.gz`)
  - **macOS (Darwin)**: `amd64`, `arm64` (packaged as `.tar.gz`)
- Automatic injection of version metadata:

  ```powershell
  .\bg3-guard.exe --version
  # Output: bg3-guard v1.0.0 (commit: abcdef1, date: 2026-09-16, built by: goreleaser)
  
  .\bg3-guard.exe version
  # Output:
  # bg3-guard v1.0.0
  # Commit:   abcdef1
  # Built at: 2026-09-16
  # Built by: goreleaser
  ```

### 2. Package Managers Configured

- **Homebrew Casks (`homebrew_casks`)**: Targets `suderio/homebrew-tap` (`brew install suderio/tap/bg3-guard`).
- **Scoop App Manifest (`scoops`)**: Targets `suderio/scoop-bucket` (`scoop install bg3-guard`).
- **Winget Package (`winget`)**: Package identifier `suderio.bg3guard` targeting `suderio/winget-pkgs`.
- **Arch User Repository (`aurs`)**: Generates PKGBUILD for `bg3-guard-bin`.
- **Linux Packages (`nfpms`)**: Generates native `.deb` and `.rpm` packages with docs and license installed to `/usr/share/`.
- All package manager uploads have fallback guards (`skip_upload: "{{ eq .Env.<TOKEN> \"\" }}"`), ensuring releases succeed even before every single token or repository is created.

### 3. Cryptographic Signing (Cosign)

- Checksums and release artifacts are signed keylessly in GitHub Actions using Cosign via Sigstore and GitHub OIDC (`id-token: write`).
- Users can verify release authenticity with:

  ```bash
  cosign verify-blob \
    --certificate checksums.txt.pem \
    --signature checksums.txt.sig \
    --certificate-identity-regexp "https://github.com/suderio/bg3-guard/" \
    --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
    checksums.txt
  ```

### 4. Social Media & Announcement Channels

- Broadcasters are configured in `.goreleaser.yaml` under `announce:`:
  - **Discord**: Webhook announcement with rich text.
  - **Bluesky**: Post with tag and release notes URL.
  - **Mastodon**: Toot with gaming and Baldur's Gate 3 tags.
  - **Twitter / X**: Tweet with release URL.
  - **Telegram**: Channel/Group alert.
  - **Reddit**: Post to target subreddit (e.g. `r/BaldursGate3`).
  - **Generic Webhook**: JSON payload for Zapier / Make / n8n automation.
- Each broadcaster is dynamically enabled when its corresponding environment variable is non-empty.

---

## Verification & Testing

### 1. Test Suite Verification

Executed `go test ./...` across all packages:

```text
?       bg3-guard                   [no test files]
?       bg3-guard/cmd               [no test files]
ok      bg3-guard/internal/backup   (cached)
ok      bg3-guard/internal/config   (cached)
ok      bg3-guard/internal/filelock (cached)
ok      bg3-guard/internal/process  (cached)
ok      bg3-guard/internal/recover  (cached)
ok      bg3-guard/internal/restore  (cached)
?       bg3-guard/internal/tui      [no test files]
```

### 2. Multi-Target Compilation Test

Simulated GoReleaser matrix compilation across all 6 target pairs:

```powershell
Compiling bg3-guard-windows-amd64.exe...
Compiling bg3-guard-windows-arm64.exe...
Compiling bg3-guard-linux-amd64...
Compiling bg3-guard-linux-arm64...
Compiling bg3-guard-darwin-amd64...
Compiling bg3-guard-darwin-arm64...
All targets compiled successfully!
```

### 3. Version Flag & Output Test

Tested built binary flags:

- `.\bg3-guard.exe --version` -> `bg3-guard v1.0.0 (commit: abcdef1, date: 2026-09-16, built by: goreleaser)`
- `.\bg3-guard.exe version` -> formatted commit, date, and builder details.

---

## Next Steps for You

See [docs/release-guide.md](../docs/release-guide.md) for full instructions:

1. **Push your initial commit** to GitHub:

   ```bash
   git add .
   git commit -m "feat: configure goreleaser, packages, and license"
   git push -u origin main
   ```

2. **Add GitHub Secrets** for any package managers and social channels you wish to activate (e.g. `HOMEBREW_TAP_GITHUB_TOKEN`, `SCOOP_BUCKET_GITHUB_TOKEN`, `DISCORD_WEBHOOK_ID`, `DISCORD_WEBHOOK_TOKEN`, etc.).
3. **Trigger a release** by tagging:

   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```
