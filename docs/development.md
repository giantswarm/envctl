# Developing envctl

This document provides guidelines and information for developers contributing to the envctl project.

## Prerequisites

*   Go 1.21+
*   Make
*   Docker (for `act` testing)
*   [act](https://github.com/nektos/act#installation) (optional, for local workflow testing)
*   Python 3+ & pip (for `yamllint`)
*   `yamllint` (`pip install yamllint`)
*   `golangci-lint` (can be installed via the CI workflow or locally: [Install Instructions](https://golangci-lint.run/usage/install/#local-installation))

## Development Setup

1. Clone the repository:
   ```zsh
   git clone https://github.com/giantswarm/envctl.git
   cd envctl
   ```

2. Install Go dependencies:
   ```zsh
   go mod download
   ```

3. Build the binary:
   ```zsh
   make build
   ```

4. Install locally for testing:
   ```zsh
   make install
   ```

## Linting and Testing Strategy

*   **Go Linting:** Handled by CircleCI (not run in this repository's GitHub Actions workflows).
*   **YAML Linting:** Performed using `yamllint`. Run locally using `make lint-yaml` or `make check` (which is also run in the CI workflow).
*   **Go Unit Tests:** Not currently run via Make targets or the GitHub Actions workflows in this repository. They should be run manually (`go test ./...`) or handled by a separate CI/CD process if configured (e.g., CircleCI).
*   **Release Dry-Run:** The GoReleaser configuration and build process can be tested using `make release-dry-run`. This is also run automatically in the CI workflow for pull requests.

## Automated Release Process

The envctl project uses GitHub Actions to automatically create releases when pull requests are merged to the main branch. Here's how it works:

1. When a PR is merged to `main`, the `.github/workflows/auto-release.yaml` GitHub Action workflow is triggered.
2. The workflow:
   * Gets the latest version tag (defaulting to v0.0.0).
   * Creates a new version by incrementing the patch number (e.g., v0.1.0 -> v0.1.1).
   * Creates and pushes a new git tag corresponding to the new version.
   * Uses GoReleaser (`goreleaser release --clean`) to build binaries, create archives, generate checksums, and create a GitHub Release.
   * The release notes (changelog) are automatically generated by GoReleaser using GitHub's API based on merged PRs since the last tag (`changelog.use: github-native`).

## Testing the Release Process

You can test parts of the release process locally using the following commands:

```zsh
# Test the GoReleaser build/archive/checksum steps without publishing
make release-dry-run

# Create a full release locally (builds, archives, checksums, GitHub release)
# Requires a GITHUB_TOKEN environment variable with repo scope.
# Note: This creates a real release and should generally only be used by maintainers for specific testing.
make release-local
```

### Testing the GitHub Actions Workflows Locally with `act`

You can use [`act`](https://github.com/nektos/act#installation) to simulate the GitHub Actions workflows locally. This requires Docker. Use the following `make` targets:

1. **Test the CI Workflow (Pull Request Event):**
   ```zsh
   # This simulates the checks run on a PR (yamllint, goreleaser dry-run)
   make test-ci-pr
   ```

2. **Test the CI Workflow (Push Event):**
   ```zsh
   # This simulates the checks run on a push to main (yamllint)
   make test-ci-push
   ```

3. **Test the Auto-Release Workflow:**
   ```zsh
   # This simulates the full release process
   make test-auto-release
   ```
   * **Note:** Requires `merged_pr_event.json` in the project root. You can create one based on the [GitHub pull_request event docs](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request) (ensure `.pull_request.merged` is `true`).
   * **Note:** The step involving `git push` for the tag might fail locally if the tag already exists or due to credential issues. You also won't see the GitHub Release creation locally.

## Contributing

1. Create a new branch for your changes:
   ```zsh
   git checkout -b feature/your-feature-name
   ```

2. Make your changes and commit them with meaningful commit messages.

3. Ensure your changes pass local checks (e.g., `make check`). Run Go linters and unit tests separately if they are not covered by other CI.

4. Push your branch and create a pull request targeting the `main` branch.

5. The CI workflow (`ci.yaml`) will run YAML linting checks (`make check`) and a GoReleaser dry-run on your PR.

6. Ensure your PR includes any necessary updates, but **do not manually update `CHANGELOG.md`**. The changelog will be generated automatically during the release.

7. Once your PR is approved and merged, the auto-release workflow will run and create a new release with generated notes.

## Version Management

The version number follows Semantic Versioning (MAJOR.MINOR.PATCH).

* The automated release process currently **only increments the PATCH version** for each merged PR.
* For **MINOR** or **MAJOR** version bumps, manual intervention is currently required after merging the relevant feature/breaking change PRs:
  1. Create and push the desired tag manually (e.g., `git tag v1.0.0`, `git push origin v1.0.0`).
  2. Manually trigger the `auto-release.yaml` workflow via the GitHub Actions UI, selecting the `main` branch and providing the manually created tag.
  *Alternatively, adjust the `Determine Next Version` step in `auto-release.yaml` temporarily before merging the PR that should trigger the bump, or create a separate release PR.*
