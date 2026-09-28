# Releasing

How KWU ships, who can ship what, and the canary-before-stable flow.

## Branch model: trunk + tags

- **`main-v3`** is the single development line (the v2 / 1.x trunk). Every PR merges here.
- **Production is a tag, not a branch.** A release is a tagged snapshot of `main-v3`:
  `v1.4.0` (CLI archives + checksums) or annotated `desktop-v1.4.0` (desktop source
  and the release-only repository's GitHub Release).
- **`v1`** is the archived 1.0/legacy line — maintenance only.
- **Hotfix** an already-released version by branching from its tag, fixing, and tagging again.

There is no separate "production" or "develop" branch by design — the canary channel
provides the pre-release buffer instead of a long-lived branch.

## Channels

| Surface | Stable | Pre-release buffer |
|---|---|---|
| CLI | GitHub release archives + checksums | `v*-rc*` GitHub prerelease |
| Desktop | OSS stable pointer, GitHub Release fallback | Not published in phase one |

Desktop phase one has no canary release. CLI prereleases keep their existing flow.

## Who can release what

| Action | Who | Mechanism |
|---|---|---|
| **Ship desktop stable** | **KWU release owner** | The owner freezes an annotated source tag and manually dispatches the Draft workflow. Actions builds the macOS asset; the fixed Windows release machine builds, Authenticode-signs, and uploads Windows assets. The owner then runs the zero-write DryRun and manually dispatches publication. |
| **Ship CLI** | Existing CLI release owners | Existing CLI tag and workflow process |

Desktop workflows do not run for ordinary PRs or branch pushes. Only an annotated
`desktop-vX.Y.Z` tag reachable from `main-v3` can be selected by the manually
dispatched `Desktop build draft` workflow; creating or pushing the tag alone does
not start a build. Formal publication always requires a separate `Desktop publish`
dispatch and exact confirmation text.

## The release loop

1. **Develop** — PRs land on `main-v3` (branch auto-deletes on merge).
2. **Test** desktop changes from normal development builds. If a CLI prerelease is
   needed, use an explicit `v1.4.0-rc.1` tag.
3. **Fix** on `main-v3` via PRs.
4. **Ship CLI stable** with the existing CLI tag:
   ```sh
   git tag v1.4.0 && git push origin v1.4.0
   ```
5. **Ship desktop stable** with the [desktop release manual](releases/desktop.md): commit
   `docs/releases/vX.Y.Z.md`, freeze the resulting `main-v3` commit with annotated
   `desktop-vX.Y.Z`, manually run `Desktop build draft`, and let Actions build and
   upload the selected macOS asset. On the fixed Windows release machine, run
   `BuildWindows` to build, Authenticode-sign, and upload the selected Windows assets.
   After real-device acceptance, run `Finalize -DryRun`, review the Draft, and manually
   run `Desktop publish`. Local non-DryRun `Finalize` is the full publication fallback.
   It is operator-exclusive: never overlap it with Actions `Desktop publish`, Actions
   `Desktop withdraw`, or another local non-DryRun `Finalize` run.
6. **Next cycle** continues on `main-v3`.

## Notes

- A CLI `-rc` tag (e.g. `v1.4.0-rc.1`) creates a prerelease GitHub release.
- Desktop in-app updates use the current platform's OSS
  `channels/stable/<target>/latest.json` first and the matching versioned GitHub
  `latest-<target>.json` as fallback. The manifest is the only source for version and
  verified assets. Matching non-empty cloud notes are preferred; missing, empty,
  timed-out, or mismatched cloud notes fall back to the Markdown snapshot in the
  manifest. The legacy global pointers remain frozen at the bootstrap migration version.
- Windows applies the downloaded, minisign-verified installer. Phase-one macOS
  builds remain manual because there is no Developer ID; Linux is not published.
