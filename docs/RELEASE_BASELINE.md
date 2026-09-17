# Release Baseline

This document records the release facts used to normalize the dependency baseline on 2026-09-17. It is deliberately separate from the source checkout's feature documentation: tag names are immutable release identities, not labels that can be reassigned to make a branch appear current.

## Authoritative published artifact

| Field | Verified value |
| --- | --- |
| Module path | `github.com/umesh0492/go-libs` |
| Published version | `v0.2.1` |
| Remote annotated-tag object | `6df2ebb2d229dd82b2f95a6c9cc5a6b3b4542b43` |
| Tagged commit | `b75acc6d82e47189ef174a4ea80134fed8cd392f` |
| Commit subject | `fix(security): rename sanitizer to redactHeaderForLogging for CodeQL recognition` |
| Go module proxy origin | `https://proxy.golang.org/github.com/umesh0492/go-libs/@v/v0.2.1.info` |
| Proxy module zip SHA-256 | `d519b6624138503f1cbe8f3de071f0fe685b521ad5f1f9634a0a85a1992fc9db` |
| Proxy `.mod` SHA-256 | `3625f2188bcc9a45a7bbf32241641dd317890d5a44c0bd32748cf12d6eb4ac47` |
| Checksum database module hash | `h1:9Fzm2GZMkd+rnFAFOd5MURnOorqM98O4H/zcVUb6uoY=` |
| Checksum database `go.mod` hash | `h1:R7gQaadUNwpnavd5P96ThNbhYyUUKyVbfKCR/mu29/o=` |

`go mod download -json github.com/umesh0492/go-libs@v0.2.1` is the independent module-resolution check. `scripts/verify_release_baseline.sh` revalidates this historical remote-tag, proxy, and checksum-database evidence. It is an audit only and is deliberately not a recurring release-workflow gate. `scripts/verify_release.sh <tag>` validates the actual immutable tag being released and emits `release-manifest.json`. The recorded published release notes are in the immutable artifact at [`v0.2.1`](https://github.com/Abeta-dev/go-libs/releases/tag/v0.2.1), rather than this checkout's unreleased changelog section.

## Checkout status

The baseline checkout starts at `3d5f640b93223231f1b4c18e6519f279059561ad`, whose nearest reachable release is `v0.1.0` (`v0.1.0-2-g3d5f640`). The published `v0.2.1` commit is not an ancestor of this checkout; the two histories must not be conflated.

This local clone also contains a conflicting pre-existing `v0.2.1` annotated tag object (`295c77298fb4f9ca80223805b516969e81985af9`) resolving to `f5f80f420615b4649e35626424f5682245c7a94d`. The verifier intentionally consults `origin` and the public module/checksum services rather than treating that local ref as authoritative. It reports the mismatch but never alters the local tag. Use a fresh clone or the recorded commit when checking out the published artifact.

## Release policy

- Do not move, recreate, or republish an existing version or tag. The release workflow treats an existing GitHub release as an already-published artifact and exits without mutation.
- Every release workflow run validates the selected tag, its exact remote target, the Go proxy archive and module file, and Go's resolved sums. It writes a retained `release-manifest.json` workflow artifact whether verification passes or fails.
- The source and API differences between the published `v0.2.1` line and this checkout require reconciliation before publishing this line. A future release from the reconciled branch must use a new monotonic **pre-1.0 minor** version, starting at `v0.3.0`; a `v0.2.2` patch would incorrectly imply compatibility with `v0.2.1`.
- The `go 1.26.0` directive is the supported toolchain baseline. Historical benchmark measurements retain their recorded toolchain so that they remain reproducible measurements rather than claims about current validation.
