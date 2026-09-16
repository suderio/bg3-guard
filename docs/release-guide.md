# BG3 Guard Release & Automation Guide

This guide details how **BG3 Guard** (`bg3-guard`) automates cross-platform builds, packages (Homebrew, Scoop, Winget, AUR, DEB, RPM), cryptographic signing, GitHub Releases, and social media announcements using **GoReleaser** and **GitHub Actions**.

---

## 1. Pipeline Overview

When you push a Git tag formatted as `v*.*.*` (e.g., `v1.0.0`):

1. **GitHub Actions** triggers `.github/workflows/release.yml`.
2. **Cosign** is installed for keyless artifact signing using GitHub OIDC tokens.
3. **Automated Tests** (`go test -v ./...`) run to ensure stability.
4. **GoReleaser v2** runs:
   - Cross-compiles binaries for **Windows**, **Linux**, and **macOS (Darwin)** across **amd64** and **arm64** architectures.
   - Injects release metadata (`Version`, `Commit`, `Date`, `BuiltBy`) into `cmd.Version`.
   - Packages archives (`.zip` for Windows, `.tar.gz` for Linux/macOS).
   - Generates `.deb` (Debian/Ubuntu) and `.rpm` (Fedora/RHEL) packages via **nfpms**.
   - Calculates SHA256 checksums in `checksums.txt`.
   - Keylessly signs `checksums.txt` using **Cosign**.
   - Publishes all assets to **GitHub Releases**.
   - Generates and pushes manifests for:
     - **Homebrew Cask** (`suderio/homebrew-tap`)
     - **Scoop Bucket** (`suderio/scoop-bucket`)
     - **Winget Manifest** (`suderio/winget-pkgs`)
     - **Arch User Repository (AUR)** (`bg3-guard-bin`)
   - Broadcasts the new version to configured social media and messaging channels.

---

## 2. Initial Setup: Pushing to GitHub

Because the remote repository at `https://github.com/suderio/bg3-guard.git` is currently empty, initialize Git and push the initial commit:

```powershell
# 1. Initialize git locally (if not already done)
git init -b main

# 2. Add remote
git remote add origin https://github.com/suderio/bg3-guard.git

# 3. Stage and commit
git add .
git commit -m "feat: initial commit with honour mode protection and goreleaser pipeline"

# 4. Push to main branch
git push -u origin main
```

---

## 3. Configuring Package Managers

GoReleaser automatically skips publishing to any package manager whose token or key is not set. You can set them up progressively:

### A. Homebrew Tap (`homebrew_casks`)

1. Create a public repository named `homebrew-tap` under your GitHub account (`https://github.com/suderio/homebrew-tap`).
2. Generate a GitHub Personal Access Token (PAT) with `repo` scope.
3. In `suderio/bg3-guard` settings (`Settings -> Secrets and variables -> Actions`):
   - Add Secret: `HOMEBREW_TAP_GITHUB_TOKEN` with the PAT value.

Users will be able to install via:

```bash
brew install suderio/tap/bg3-guard
```

### B. Scoop Bucket (`scoops`)

1. Create a public repository named `scoop-bucket` under your GitHub account (`https://github.com/suderio/scoop-bucket`).
2. Use the same GitHub PAT or create a dedicated one with `repo` scope.
3. In `suderio/bg3-guard` settings:
   - Add Secret: `SCOOP_BUCKET_GITHUB_TOKEN`.

Users will be able to install via:

```powershell
scoop bucket add suderio https://github.com/suderio/scoop-bucket
scoop install bg3-guard
```

### C. Winget Manifest (`winget`)

1. Create a repository named `winget-pkgs` or fork `microsoft/winget-pkgs`.
2. Generate a GitHub PAT with `repo` scope.
3. In `suderio/bg3-guard` settings:
   - Add Secret: `WINGET_GITHUB_TOKEN`.

### D. Arch User Repository (`aurs`)

1. Register an account on [aur.archlinux.org](https://aur.archlinux.org/) and register an SSH public key.
2. Generate a dedicated SSH key pair for CI:

   ```bash
   ssh-keygen -t ed25519 -C "aur@github-actions" -f ./aur_key -N ""
   ```

3. Add the public key (`aur_key.pub`) to your AUR profile.
4. Add the private key content (`aur_key`) as a GitHub Secret:
   - Secret Name: `AUR_KEY`.

Arch users will be able to install via `yay` or `paru`:

```bash
yay -S bg3-guard-bin
```

---

## 4. Configuring Social Media Announcers

All announcers in `.goreleaser.yaml` are guarded by conditional checks (`enabled: "{{ ne .Env.<VAR> \"\" }}"`). Only the services you provide credentials for will broadcast:

| Platform | Required GitHub Secret(s) | Description |
| :--- | :--- | :--- |
| **Discord** | `DISCORD_WEBHOOK_ID` and `DISCORD_WEBHOOK_TOKEN` | Created from Discord Server Channel Settings -> Integrations -> Webhooks |
| **Bluesky** | `BLUESKY_IDENTIFIER` and `BLUESKY_APP_PASSWORD` | Handle (e.g. `user.bsky.social`) & App Password from Bluesky Settings |
| **Mastodon** | `MASTODON_SERVER` and `MASTODON_CLIENT_TOKEN` | Instance URL (e.g. `https://mastodon.social`) and OAuth Access Token |
| **Telegram** | `TELEGRAM_CHAT_ID` and `TELEGRAM_TOKEN` | Channel/Group ID and Bot token from `@BotFather` |
| **Twitter / X** | `TWITTER_CONSUMER_KEY`, `TWITTER_CONSUMER_SECRET`, `TWITTER_ACCESS_TOKEN`, `TWITTER_ACCESS_TOKEN_SECRET` | Developer App credentials from Twitter Developer Portal |
| **Reddit** | `REDDIT_APP_ID`, `REDDIT_APP_SECRET`, `REDDIT_USERNAME`, `REDDIT_PASSWORD` | Script application credentials created at reddit.com/prefs/apps |
| **Webhooks** | `SOCIAL_BROADCAST_WEBHOOK_URL` | Generic webhook endpoint (Zapier / Make / n8n) for custom workflows |

---

## 5. Cryptographic Signing (Cosign)

Releases are signed using **Cosign** keyless signing powered by Sigstore and GitHub OIDC. No private keys or passwords need to be stored in GitHub Secrets.

Anyone can verify the authenticity and provenance of a downloaded release using:

```bash
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp "https://github.com/suderio/bg3-guard/" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  checksums.txt
```

---

## 6. How to Cut a New Release

1. Ensure all changes are committed and pushed to `main`:

   ```bash
   git status
   git push origin main
   ```

2. Create an annotated Git tag matching semantic versioning:

   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0: Initial public release"
   ```

3. Push the tag to GitHub:

   ```bash
   git push origin v1.0.0
   ```

4. Watch the progress in the **Actions** tab of your GitHub repository. Within 2-3 minutes:
   - The GitHub Release with binary assets and release notes will be live.
   - Package manager manifests will be pushed.
   - Announcements will be sent to all configured social platforms.
