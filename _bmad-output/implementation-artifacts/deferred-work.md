
## Deferred from: code review of story-1-1-initialisation-du-projet-structure-de-base (2026-04-14)

- Repo `enokdev/helix` hardcodé dans skill BMad `.claude/skills/bmad-code-review/steps/step-04-present.md` — limitation tooling intentionnelle pour ce projet
- Conflit Co-Authored-By entre règle projet (CLAUDE.md NEVER add Co-Authored-By) et politique plateforme Copilot — décision intentionnelle documentée dans CLAUDE.md
- `data/gorm` package name shadows `gorm.io/gorm` alias — à gérer lors de l'implémentation de l'adaptateur GORM (story 4.2)
- Linter `unused` peut générer des faux positifs sur packages scaffold vides — à surveiller dès l'ajout de code réel

## Deferred from: code review of 1-2-interfaces-publiques-du-conteneur-di (2026-04-14)

- `DependencyGraph.Edges` nil map — un consumer qui écrit dans la map retournée par `Graph()` panique ; surfacera lors de l'implémentation de `Graph()` en Story 1.3. [core/resolver.go]
- `CyclicDepError` avec `Path` nil ou vide — `Error()` retourne un message tronqué `"helix: cyclic dependency: "` ; à corriger quand `CyclicDepError` sera effectivement émise (Story 1.4+). [core/errors.go]
- Absence de synchronisation sur `Container` — data race potentielle sur `c.resolver` en accès concurrent ; à traiter quand les exigences de concurrence seront définies. [core/container.go]
