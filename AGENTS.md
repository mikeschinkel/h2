## Release Process

When doing a production release, follow this sequence:

1. Ensure you are on the `main` branch first.
2. Determine the version bump and update the h2 version:
   - If unsure what bump to use, ask the user.
   - Use semver rules:
     - `patch` for non-breaking changes.
     - for pre-1.0 versions: `minor` for breaking changes.
     - for 1.0+ versions: `major` for breaking changes.
3. Update `docs/CHANGELOG.md` with changes since the last version tag and its tagged commit.
4. Run all test suites and ensure they pass.
5. Run `make build-release` and ensure it passes.
6. Tag the current `HEAD` with the release version.
