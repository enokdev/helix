# HTTP Layer Documentation Implementation Guide

This guide provides the sequence and specific edits needed to apply the 9 findings from `HTTP_LAYER_CLARIFICATIONS.md` to `docs/http-layer.md`.

## Applying Changes in Recommended Order

### Step 1: Finding 3 — Query Parameter Binding Order (HIGHEST PRIORITY)

**Location:** Section "## Extracteurs types" → "### Query params" (lines 162-182)

**Action:** Replace the entire "### Query params" subsection with the expanded version from Finding 3 in the clarifications document.

**Key changes:**
- Add numbered process flow (Extraction → Default → Conversion → Max Check → Validation)
- Create table of supported tags with descriptions
- Add "Codes d'erreur" table distinguishing `INVALID_QUERY_PARAM` vs `VALIDATION_FAILED`
- Add code example showing all three error cases

**Validation after edit:**
```bash
grep -A 5 "Extraction.*Defaut.*Conversion" docs/http-layer.md
grep "INVALID_QUERY_PARAM" docs/http-layer.md
grep "VALIDATION_FAILED" docs/http-layer.md
```

---

### Step 2: Finding 4 — JSON Body Binding Validation (HIGHEST PRIORITY)

**Location:** Section "## Extracteurs types" → "### Body JSON" (lines 184-199)

**Action:** Replace the "### Body JSON" subsection with the expanded version from Finding 4.

**Key changes:**
- Add 5 numbered validation rules
- Add "Exemples:" with ✅/❌ cases
- Clarify `{}` is valid but `""` and `   ` are not
- Add "Codes d'erreur" section

**Validation after edit:**
```bash
grep -A 3 "Regles de validation du body" docs/http-layer.md
grep "DisallowUnknownFields" docs/http-layer.md
grep "multiple valeurs JSON" docs/http-layer.md
```

---

### Step 3: Finding 6 — Error Handler Signatures (HIGH PRIORITY)

**Location:** Section "## Error handlers" (lines 334-374)

**Action:** 
1. Keep existing description intro
2. Replace example code block with the two-method example from Finding 6
3. Add "**Signature requise:**" subsection with detailed rules
4. Add "**Contraintes:**" subsection
5. Add "**Invocation:**" subsection explaining error handler matching via `errors.As`

**Key changes:**
- Show example with both `(ErrorType)` and `(ctx, ErrorType)` signatures
- Clarify return type is `(any, int)` not error
- Explain duplicate detection during registration
- Add example custom error type and handler

**Validation after edit:**
```bash
grep "(any, int)" docs/http-layer.md
grep "errors.As" docs/http-layer.md
grep "Signature requise" docs/http-layer.md
```

---

### Step 4: Finding 2 — Guard Factory Init-Time Errors (HIGH PRIORITY)

**Location:** Section "## Guards" → After factory registration example (after line 240)

**Action:** Add paragraph after the factory example explaining init-time error handling.

**Key changes:**
- Add bold note about `RegisterController` failing on factory error
- Clarify errors are init-time, not request-time
- Add code example showing correct factory pattern

**Insertion point:** After the `web.RegisterGuardFactory` code block, before "Puis :"

```markdown
> **Important**: Si une factory de guard retourne une erreur, ...
```

**Validation after edit:**
```bash
grep -B 2 -A 2 "RegisterController echoue" docs/http-layer.md
grep "init-time" docs/http-layer.md
```

---

### Step 5: Finding 5 — Guard/Interceptor Execution Order (MEDIUM PRIORITY)

**Location:** Section "## Guards" → Last paragraph (around line 253)

**Action:** Replace the paragraph starting with "Les guards globaux passent..." with expanded version from Finding 5.

**Key changes:**
- Add "**Important**: Les guards executent **avant** les interceptors"
- Add numbered execution flow (1-5)
- Clarify guard failure prevents downstream execution

**Validation after edit:**
```bash
grep -A 6 "Les guards executent" docs/http-layer.md
grep "Interceptors.*du premier declare" docs/http-layer.md
```

---

### Step 6: Finding 1 — Handler nil,nil Response (MEDIUM PRIORITY)

**Location:** Section "## Mapping des reponses" → After status codes paragraph (around line 305)

**Action:** Add note paragraph after the success explanation.

**Key changes:**
- Explain nil serializes as `null` in JSON
- Show both nil and empty slice examples
- Clarify when to use each pattern

**Insertion point:** After "le body JSON est direct, sans wrapper `data`."

```markdown
> **Note sur les reponses sans payload**: Si un handler...
```

**Validation after edit:**
```bash
grep -A 3 "Note sur les reponses sans payload" docs/http-layer.md
grep "null" docs/http-layer.md
```

---

### Step 7: Finding 7 — Nested Struct Binding (LOW PRIORITY)

**Location:** After "### Body JSON" section (around line 199)

**Action:** Add "Note sur les structs imbriquees" paragraph.

**Key changes:**
- Explain only top-level exported fields are bound
- Show bad vs good struct examples
- Note about silent ignoring of nested fields

**Insertion point:** After the "Ne melangez pas..." paragraph

**Validation after edit:**
```bash
grep "champs imbriques" docs/http-layer.md
grep "silencieusement" docs/http-layer.md
```

---

### Step 8: Finding 8 — Reserved Interceptor Names (LOW PRIORITY)

**Location:** Section "## Interceptors" → Last paragraph (around line 296)

**Action:** Modify the sentence about `cache` to include other reserved names.

**Current text:**
```
N'enregistrez pas un autre interceptor sous le nom `cache` : le serveur le reserve deja.
```

**New text:**
```
N'enregistrez pas un autre interceptor sous le nom `cache` : le serveur le reserve deja pour l'interceptor integre. Evitez aussi les noms qui pourraient confluer avec des starters (ex: `auth`, `security`, `cors`).
```

**Validation after edit:**
```bash
grep "confluer avec des starters" docs/http-layer.md
```

---

### Step 9: Finding 9 — Directive Syntax Edge Cases (MEDIUM PRIORITY)

**Location:** Section "## Routes custom" → Format constraints (around line 114-119)

**Action:** Expand the constraint list in "Le format est strict :" bullet point.

**Current:**
```
- deux slashes, sans espace : `//helix:route GET /users/search`;
- exactement une methode HTTP et un chemin;
- le chemin commence par `/`;
```

**New (expanded):**
```
- deux slashes, sans espace : `//helix:route GET /users/search` (les espaces multiples entre METHOD et PATH sont normalises par le parser)
- exactement une methode HTTP et un chemin, separes par un ou plusieurs espaces
- la methode doit etre valide (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS, TRACE, CONNECT)
- le chemin doit commencer par `/` (ex: `/users`, `/`, `/users/:id`)
- les chemins avec tirets `/users-search` et slashs `/users/search` sont valides
- les chemins avec parametres `:id` et chemins finissant par `/` (ex: `/users/`) sont valides
```

And add "Exemples invalides:" section with 7-8 examples from Finding 9.

**Validation after edit:**
```bash
grep "espaces multiples" docs/http-layer.md
grep "Exemples invalides" docs/http-layer.md
```

---

## Test Improvements (documentation_test.go)

**Location:** Function `TestHTTPLayerGuideDocumentsCoreConcepts` (lines 149-213)

**Action:** Add enhanced pattern-based assertions after the existing substring checks.

**Code to add (before the closing brace):**

```go
// Enhanced validation for complex behaviors
patterns := []struct {
    name    string
    pattern string
    isRegex bool
}{
    {"nil response behavior", "nil.*null", false},
    {"guard factory init-time errors", "RegisterController.*echoue", false},
    {"query binding order process", "Extraction.*Defaut.*Conversion", true},
    {"error codes distinction", "INVALID_QUERY_PARAM.*VALIDATION_FAILED", true},
    {"JSON body validation rules", "DisallowUnknownFields", false},
    {"guard execution before interceptors", "guards executent.*avant.*interceptors", true},
    {"error handler return signature", `\(any, int\)`, true},
}

for _, p := range patterns {
    if p.isRegex {
        if !regexp.MustCompile(p.pattern).MatchString(guide) {
            t.Errorf("docs/http-layer.md should match pattern %q for %s", p.pattern, p.name)
        }
    } else {
        if !strings.Contains(guide, p.pattern) {
            t.Errorf("docs/http-layer.md should contain %q for %s", p.pattern, p.name)
        }
    }
}
```

**Required imports:**
- `regexp` (add to imports if not present)

---

## Verification Checklist

After applying all changes:

- [ ] All 9 findings have been applied to docs/http-layer.md
- [ ] No syntax errors in markdown (balanced backticks, proper headers)
- [ ] All French text (no English mixed in main body)
- [ ] All code examples have proper Go syntax
- [ ] All inline code is properly wrapped in backticks
- [ ] All tables are properly formatted
- [ ] Test updates applied to documentation_test.go
- [ ] Run `go test ./... -run Documentation` passes

**Final validation command:**
```bash
go test ./... -run TestHTTPLayerGuideDocumentsCoreConcepts -v
```

---

## Summary

- **9 Findings** across HTTP layer documentation
- **~154 lines** of new content and expansions
- **3 Priority levels**: HIGH (6), MEDIUM (2), LOW (1)
- **Test improvements**: 6-8 new pattern-based assertions
- **No breaking changes**: All updates are clarifications and additions
