# NovaPanel upstream update process

NovaPanel remains connected to `MHSanaei/3x-ui`, but its release tags are
independent. Never fetch upstream tags into this clone: upstream and NovaPanel
both use names such as `v3.5.0` and `dev-latest`.

## Recommended update

1. In GitHub Actions, run **Prepare upstream sync**.
2. Review the pull request it creates. If upstream changed NovaPanel branding,
   install URLs, Docker image names, or release workflows, keep the NovaPanel
   versions.
3. Wait for CI, CodeQL, docs, and release build checks to pass.
4. Merge the pull request.
5. Bump `internal/config/version`, commit, create the matching tag, and push it.
   The release workflow refuses tags that do not exactly match the file.

## Local fallback

```bash
git status --short
git branch backup/pre-upstream-sync-YYYYMMDD
git fetch upstream main --no-tags
git merge --no-ff upstream/main
```

Resolve conflicts, then run the full checks before pushing. Do not use
`git fetch --all --tags`: the rolling `dev-latest` tag and overlapping stable
tags are expected to collide.

## Publish

For version `3.5.3`:

```bash
printf '3.5.3\n' > internal/config/version
git add internal/config/version
git commit -m "chore(release): set version to 3.5.3"
git push origin main
git tag -a v3.5.3 -m "NovaPanel v3.5.3"
git push origin v3.5.3
```

The stable release stays draft until all Linux and Windows packages are built,
executed under their target architecture, and a `SHA256SUMS` manifest is
generated. Docker publication separately verifies the five-platform manifest
and starts every image under QEMU.
