# Repository automation

## Dependabot auto-merge

Dependabot opens a pull request; `.github/workflows/dependabot-auto-merge.yaml`
merges it once every check on that commit has passed, and leaves it alone
otherwise. Nothing to configure — it works as committed.

### Why it does not use GitHub's auto-merge

The usual recipe is `gh pr merge --auto`, which delegates the decision to GitHub:
merge when the *required status checks* pass. Two reasons that is not what runs
here.

**It is unavailable on this repository.** `buffers` is private and
`the-protobuf-project` is on the free plan, where branch protection, rulesets and
the native auto-merge feature are all withheld. The API returns:

```
403: Upgrade to GitHub Pro or make this repository public to enable this feature.
```

`--auto` would fail on every run.

**And it degrades quietly where it *is* available.** "Required status checks"
means required *by branch protection*. On a repository with protection that does
not name these checks, a pull request has no required checks, so it is mergeable
the instant it opens and `--auto` merges it before CI has finished. The
automation would look like it was gating on CI while gating on nothing.

So the workflow reads the check results itself. That behaves identically before
and after this repository becomes public, and there is no configuration that can
silently turn the gate off.

### What has to be green

Every check reported for the commit — not a list someone has to maintain. A list
has to be remembered, and the failure mode of forgetting is that auto-merge
quietly stops covering the job you just added. Requiring the whole set inverts
that: a new CI job gates auto-merge the moment it exists.

`skipped` and `neutral` count as passing, since a conditional job that did not
run has not failed.

There is one named list, `REQUIRED_CHECKS`, and it is a *floor* rather than the
gate: it names the checks that must have **reported at all**, so a commit whose CI
has not started yet — and which therefore has no failing checks — does not sail
through.

### Update types

All of them, including majors, gated on CI. Worth knowing what that means here
specifically: `protokit` is the IR engine this plugin is built on, and a major
bump of it is the dependency change most likely to alter *generated output*
rather than break the build. What stands between that and a silent change to
every consumer's schema is the golden and toolchain tests — which is why they run
on every pull request, and why the gate above requires all of them rather than a
subset.

Minor and patch updates are grouped into one weekly pull request; majors arrive
individually, so a group diff is never the place a breaking bump hides.

## When this repository goes public

Native auto-merge and branch protection become available. Neither is required —
the workflow above keeps working unchanged — but branch protection is worth
adding as defence in depth, since it also stops a *human* merging past red CI:

```sh
gh api -X PUT repos/the-protobuf-project/buffers/branches/main/protection \
  --input .github/branch-protection.json
```

The contexts in that file are the job `name:` fields the workflows report, and
they mirror `REQUIRED_CHECKS`. Verify with:

```sh
gh api repos/the-protobuf-project/buffers/branches/main/protection/required_status_checks \
  --jq '.contexts'
```

## Release

Tagging `v*` triggers two workflows:

| Workflow | Does |
|---|---|
| `release.yaml` | GoReleaser: both binaries for 6 platforms, a Homebrew cask in the org tap, a GitHub release |
| `publish.yaml` | `buf push`: the `buffers.v1` vocabulary to the BSR |

Both need secrets that are not set yet:

- `HOMEBREW_TAP_GITHUB_TOKEN` — a PAT with `repo` scope on `the-protobuf-project/homebrew-tap`
- `BUF_TOKEN` — a BSR token with write access to the organization

Add them under Settings → Secrets and variables → Actions.
