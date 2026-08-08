# Auto-approve identical permission grants

## Motivation

tpd re-prompts a permission dialog when a profile's approval hash changes.
The hash deliberately includes the contributor identity (`FullName` +
`Namespace`) of every gated grant, so approvals "don't silently transfer
across contributors with identical field values" (hash.go:16). That guard is
also the reason a harmless refactor of tpd's own built-in catalog — renaming
a fragment folder, splitting `infra/` into `cloud`+`sysutils`, moving
`docker/podman/nix` to `sysutils` — re-prompts even though the granted
permissions are byte-identical. The user found this noise not worth the
guard and chose to drop it: **auto-approve whenever the granted values are
identical, no matter which contributor provides them.**

The exposure is capped either way: values are still hashed, so any *future*
change to a grant (new mount, ro→rw flip, new env value, …) re-prompts. The
identity check only surfaced "the source of this grant changed" earlier; it
never granted a blank check.

## Context

- `ComputeApprovalHash` (internal/approval/hash.go:17) writes
  `field\nkey\ncontributorFullName\ncontributorNamespace\nvalue` for each
  non-user gated field, pre-template-expansion, truncated to 12 hex chars.
- `decide` (approval.go:214) re-prompts an item when `st.Hash != req.Hash`
  or the field has no stored decision; prior-approved keys are pre-checked.
  A stored decision set (approved keys, denied = absent) is reused only when
  the hash matches.
- `reconcileState` drops stale stored keys, but only when the hash matches.
- `Trusted()` contributors (`Namespace == ""`) are excluded from the hash
  entirely and never gated; this is unchanged.
- The hash is used only by `approval.Filter`; the prompt does not display it.
- `TestHashChangesOnContributorSwap` (hash_test.go:32) is the only test
  encoding the contributor-in-hash property. The hash-mismatch re-prompt
  tests (`TestFilterMarksPriorApprovedOnHashChange`, …) use fake stored
  hashes and assert re-prompt mechanics, which we keep.

## Design

### 1. Drop contributor identity from the hash

`ComputeApprovalHash`'s `emit` writes only `field`, `key`, and the canonical
pre-expansion value string; contributor identity is no longer part of the
hash input. The
`services` loop likewise drops `c.FullName`/`c.Namespace` from its emission.
The comment is updated to state the new contract: the hash is a fingerprint
of the granted set only, so a grant moving between contributors does not
re-prompt, while any value/key change does.

### 2. Semantics

- Same granted set as the stored approval → no prompt, decisions reused,
  regardless of which fragment/profile now contributes each grant.
- Any value or key change → whole-list re-prompt with prior-approved items
  pre-checked, exactly as today.
- **User-owned grants never prompt.** Gating is by contributor, not by
  content: `decide` short-circuits on `Trusted()` (`Namespace == ""`), the
  stamp every entry under the user's `~/.config/tpd/{profiles,fragments}/`
  carries, and the hash excludes trusted contributions at emission. This is
  unchanged by the design. The one interaction: moving a grant between a
  catalog fragment and the user's own files changes the *gated set*, so the
  hash changes and the *remaining* catalog grants re-prompt (pre-checked);
  the user-owned item itself never appears in the dialog.

No changes to `Filter`, `decide`, `reconcileState`, the `State` struct, the
state-file YAML schema, or `mergeChoicesIntoState`.

### 3. Migration

Stored approvals were hashed with contributor identity; after the change the
computed hash no longer matches them. First launch of a profile after
upgrading re-prompts once (all prior-approved items pre-checked) and
re-saves under the values-only hash. Afterwards, catalog refactors and
contributor changes never re-prompt again. No migration of existing state
files is possible or needed — the one re-prompt is the migration.

## Testing

- `TestHashChangesOnContributorSwap` is inverted to assert the hash is
  **identical** when only the contributor differs — covering both a
  core-namespace swap (`core/creds/ssh` → `core/creds/git`) and a
  core→remote swap (`core/creds/ssh` → `github.com/foo/ssh`).
- A new hash test asserts the hash **changes** when the same grant moves
  between a gated contributor and a trusted (user-owned) contributor — the
  gated set itself changed, which is the user-files interaction above.
- Existing hash tests (stability, excludes user contributions, template
  literals, service-definition value changes still alter the hash) are
  unchanged and must still pass.
- Existing `Filter`/`decide`/prompt tests are unchanged; they use fake
  stored hashes and must still pass.
- Add a `Filter`-level test proving a contributor-only change yields an
  empty `PromptRequest` (no prompt) while a value change still prompts.

## Out of scope

- Per-item re-prompt on genuine changes (only changed keys prompt). The
  whole-list pre-checked re-prompt is kept (Approach C was declined).
- Persisting the resolved values in the state file (Approach B's values
  hash is unnecessary once contributor identity leaves the hash).
