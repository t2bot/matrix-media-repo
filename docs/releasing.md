# Releasing MMR

MMR is released whenever the changelog *feels* worthy of a release.

## Prerequisites

1. Ensure `CHANGELOG.md` is up-to-date and has consistent language.
2. Ensure tests pass on main branch.

## Release

1. Update `version/version.go#DocsVersion` to point to the about-to-be-released version.
2. In the [docs.t2bot.io repo](https://github.com/t2bot/docs.t2bot.io):
   1. Rename `/content/matrix-media-repo/unstable` to `/content/matrix-media-repo/v1.3.4` (if releasing v1.3.4).
   2. Run `npm run build` (after `npm install` if required).
   3. Copy `/site/matrix-media-repo/v1.3.4` to `/old_versions/matrix-media-repo/v1.3.4`.
   4. Rename `/content/matrix-media-repo/v1.3.4` back to `/content/matrix-media-repo/unstable`.
   5. Commit and push changes.
3. Update the links and headers in `CHANGELOG.md`.
4. Commit any outstanding changes.
5. Create a git tag for `v1.3.4` and push both the tag and main branch.
6. On the GitHub releases page, create a new release for the pushed tag. The title is the tag name, and the content is the relevant changelog section.
7. Build binaries for Windows and Linux, then attach them to the GitHub release.
8. Publish the release.
9. Ensure the Docker image is built or building, then announce the release in the MMR Matrix room.
