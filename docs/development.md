# Developing envctl

This document provides guidelines and information for developers contributing to the envctl project.

## Development Setup

1. Clone the repository:
   ```zsh
   git clone https://github.com/giantswarm/envctl.git
   cd envctl
   ```

2. Install dependencies:
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

## Automated Release Process

The envctl project uses GitHub Actions to automatically create releases when pull requests are merged to the main branch. Here's how it works:

1. When a PR is merged to `main`, the `auto-release.yaml` GitHub Action workflow is triggered
2. The workflow:
   - Gets the latest version tag
   - Creates a new version by incrementing the patch number
   - Updates the CHANGELOG.md with information about the merged PR
   - Creates a new git tag
   - Uses GoReleaser to build binaries for multiple platforms
   - Creates a GitHub Release with the binaries attached

## Testing the Release Process

You can test the release process locally using the following commands:

```zsh
# Test the release process without publishing
make release-dry-run

# Create a release locally (requires GITHUB_TOKEN)
# Note: This should only be used by maintainers
make release-local
```

## Contributing

1. Create a new branch for your changes:
   ```zsh
   git checkout -b feature/your-feature-name
   ```

2. Make your changes and commit them with meaningful commit messages

3. Create a pull request targeting the `main` branch

4. Once your PR is approved and merged, a new release will be automatically created

## Version Management

The version number follows semantic versioning (MAJOR.MINOR.PATCH):

- MAJOR: Incompatible API changes
- MINOR: Backwards-compatible new features
- PATCH: Backwards-compatible bug fixes

The automated release process increments the patch version for each merged PR. For major or minor version bumps, include "major version bump" or "minor version bump" in your PR title or description.
