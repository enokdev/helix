---
deferred_work_file: '{implementation_artifacts}/deferred-work.md'
---

# Step 4: Present and Act

## RULES

- YOU MUST ALWAYS SPEAK OUTPUT in your Agent communication style with the config `{communication_language}`
- When `{spec_file}` is set, always write findings to the story file before offering action choices.
- `decision-needed` findings must be resolved before handling `patch` findings.

## INSTRUCTIONS

### 1. Clean review shortcut

If zero findings remain after triage (all dismissed or none raised): state that and proceed to section 6 (Sprint Status Update).

### 2. Write findings to the story file

If `{spec_file}` exists and contains a Tasks/Subtasks section, append a `### Review Findings` subsection. Write all findings in this order:

1. **`decision-needed`** findings (unchecked):
   `- [ ] [Review][Decision] <Title> — <Detail>`

2. **`patch`** findings (unchecked):
   `- [ ] [Review][Patch] <Title> [<file>:<line>]`

3. **`defer`** findings (checked off, marked deferred):
   `- [x] [Review][Defer] <Title> [<file>:<line>] — deferred, pre-existing`

Also append each `defer` finding to `{deferred_work_file}` under a heading `## Deferred from: code review ({date})`. If `{spec_file}` is set, include its basename in the heading (e.g., `code review of story-3.3 (2026-03-18)`). One bullet per finding with description.

### 3. Present summary

Announce what was written:

> **Code review complete.** <D> `decision-needed`, <P> `patch`, <W> `defer`, <R> dismissed as noise.

If `{spec_file}` is set, add: `Findings written to the review findings section in {spec_file}.`
Otherwise add: `Findings are listed above. No story file was provided, so nothing was persisted.`

### 4. Resolve decision-needed findings

If `decision_needed` findings exist, present each one with its detail and the options available. The user must decide — the correct fix is ambiguous without their input. Walk through each finding (or batch related ones) and get the user's call. Once resolved, each becomes a `patch`, `defer`, or is dismissed.

If the user chooses to defer, ask: Quick one-line reason for deferring this item? (helps future reviews): — then append that reason to both the story file bullet and the `{deferred_work_file}` entry.

**HALT** — I am waiting for your numbered choice. Reply with only the number (or "0" for batch). Do not proceed until you select an option.

### 5. Handle `patch` findings

If `patch` findings exist (including any resolved from step 4), HALT. Ask the user:

If `{spec_file}` is set, present all three options (if >3 `patch` findings exist, also show option 0):

> **How would you like to handle the <Z> `patch` findings?**
> 0. **Batch-apply all** — automatically fix every non-controversial patch (recommended when there are many)
> 1. **Fix them automatically** — I will apply fixes now
> 2. **Leave as action items** — they are already in the story file
> 3. **Walk through each** — let me show details before deciding

If `{spec_file}` is **not** set, present only options 1 and 3 (omit option 2 — findings were not written to a file). If >3 `patch` findings exist, also show option 0:

> **How would you like to handle the <Z> `patch` findings?**
> 0. **Batch-apply all** — automatically fix every non-controversial patch (recommended when there are many)
> 1. **Fix them automatically** — I will apply fixes now
> 2. **Walk through each** — let me show details before deciding

**HALT** — I am waiting for your numbered choice. Reply with only the number (or "0" for batch). Do not proceed until you select an option.

- **Option 0** (only when >3 findings): Apply all non-controversial patches without per-finding confirmation. Skip any finding that requires judgment. Present a summary of changes made and any skipped findings.
- **Option 1**: Apply each fix. After all patches are applied, present a summary of changes made. If `{spec_file}` is set, check off the items in the story file.
- **Option 2** (only when `{spec_file}` is set): Done — findings are already written to the story.
- **Walk through each**: Present each finding with full detail, diff context, and suggested fix. After walkthrough, re-offer the applicable options above.

  **HALT** — I am waiting for your numbered choice. Reply with only the number (or "0" for batch). Do not proceed until you select an option.

**✅ Code review actions complete**

- Decision-needed resolved: <D>
- Patches handled: <P>
- Deferred: <W>
- Dismissed: <R>

### 6. Update story status and sync sprint tracking

Skip this section if `{spec_file}` is not set.

#### Determine new status based on review outcome

- If all `decision-needed` and `patch` findings were resolved (fixed or dismissed) AND no unresolved HIGH/MEDIUM issues remain: set `{new_status}` = `done`. Update the story file Status section to `done`.
- If `patch` findings were left as action items, or unresolved issues remain: set `{new_status}` = `in-progress`. Update the story file Status section to `in-progress`.

Save the story file.

#### Sync sprint-status.yaml

If `{story_key}` is not set, skip this subsection and note that sprint status was not synced because no story key was available.

If `{sprint_status}` file exists:

1. Load the FULL `{sprint_status}` file.
2. Find the `development_status` entry matching `{story_key}`.
3. If found: update `development_status[{story_key}]` to `{new_status}`. Update `last_updated` to current date. Save the file, preserving ALL comments and structure including STATUS DEFINITIONS.
4. If `{story_key}` not found in sprint status: warn the user that the story file was updated but sprint-status sync failed.

If `{sprint_status}` file does not exist, note that story status was updated in the story file only.

#### Completion summary

> **Review Complete!**
>
> **Story Status:** `{new_status}`
> **Issues Fixed:** <fixed_count>
> **Action Items Created:** <action_count>
> **Deferred:** <W>
> **Dismissed:** <R>

### 7. Commit, push and close GitHub issue

Perform this section only when `{new_status}` = `done`.

#### Commit

Stage and commit all modified source files (exclude `_bmad-output/` — it is gitignored):

```bash
git add <source files changed during review>
git commit -m "fix(<scope>): code review patches — story <story_key>"
```

Use conventional commit format from CLAUDE.md. Never add `Co-Authored-By`.

#### Push

```bash
git push origin <current-branch>
```

#### Verify CI

After pushing, wait for CI checks to complete and verify they pass:

1. Get the SHA of the commit just pushed:
   ```bash
   git rev-parse HEAD
   ```
2. Poll until all checks are no longer pending (up to ~5 minutes, checking every 30 s):
   ```bash
   gh run list --repo enokdev/helix --branch <current-branch> --limit 1 --json status,conclusion,databaseId
   ```
3. If the run is `completed` with conclusion `success`: proceed to close the GitHub issue.
4. If the run fails or is cancelled:
   - Fetch the failing step logs:
     ```bash
     gh run view <run-id> --repo enokdev/helix --log-failed
     ```
   - Diagnose and fix the root cause in the source files.
   - Stage, commit, and push the fix using the same conventional-commit format.
   - Re-poll from step 2 until CI passes.
   - **Do not close the GitHub issue or move the project task until CI is green.**
5. If no CI run is found after 2 minutes, note it to the user and continue.

#### Close GitHub issue

1. Search for the open issue matching the story:
   ```bash
   gh issue list --repo enokdev/helix --search "Story <N.M>" --json number,title,state
   ```
2. Close it:
   ```bash
   gh issue close <number> --repo enokdev/helix --comment "Story <N.M> implémentée et validée — code review terminée. ✅"
   ```
   If no matching issue is found, note it to the user and continue.

#### Move GitHub project task

After closing the issue, move the corresponding item to **Done** in the GitHub project (`https://github.com/orgs/enokdev/projects/1`):

1. Discover the project node ID, status field ID, and "Done" option ID:
   ```bash
   gh project list --owner enokdev --format json
   gh project field-list 1 --owner enokdev --format json
   ```
2. Find the project item ID matching the story:
   ```bash
   gh project item-list 1 --owner enokdev --format json
   ```
3. Move the item to Done:
   ```bash
   gh project item-edit \
     --id <item-node-id> \
     --field-id <status-field-id> \
     --project-id <project-node-id> \
     --single-select-option-id <done-option-id>
   ```
   If the item is not found in the project, note it to the user and continue.

### 8. Next steps

Present the user with follow-up options:

> **What would you like to do next?**
> 1. **Start the next story** — run `dev-story` to pick up the next `ready-for-dev` story
> 2. **Re-run code review** — address findings and review again
> 3. **Done** — end the workflow

**HALT** — I am waiting for your choice. Do not proceed until the user selects an option.
