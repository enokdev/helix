# Lacunes Helix - Roadmap Actionnable

Ce fichier liste les prochains travaux qui ne sont pas encore couverts comme une roadmap claire apres la base v0.1.0. L'objectif est de transformer les angles morts techniques, produit et adoption en taches concretes.

## P0 - Bloquants Release et Adoption

- [x] Verifier l'installation externe depuis un projet neuf
  - Domaine: release, DX
  - Pourquoi: les tests actuels valident surtout le repo lui-meme; il faut prouver que `go get github.com/enokdev/helix` et `go install github.com/enokdev/helix/cmd/helix@latest` fonctionnent hors du checkout local.
  - Action: creer un projet temporaire vierge, installer Helix, lancer un mini serveur zero-config et verifier `helix version`.
  - Validation: un script ou une procedure reproductible passe sur une machine propre avec Go 1.21+.
  - Preuve: `scripts/smoke_external_install.sh` cree un module temporaire externe, ajoute Helix via `go get` avec `replace`, installe `cmd/helix`, verifie `helix --version`, build l'app zero-config et teste `/actuator/health`.

- [x] Tester la compatibilite Go sur plusieurs versions supportees
  - Domaine: CI, compatibilite
  - Pourquoi: la CI utilise Go 1.21, mais le developpement local se fait avec Go 1.25.5; les regressions liees aux versions peuvent passer inapercues.
  - Action: ajouter une matrice CI Go 1.21, 1.22, 1.23, 1.24 et 1.25 pour `go test ./...` et `go build ./...`.
  - Validation: la matrice passe sans modifier la directive `go 1.21.0` du module.
  - Preuve: `.github/workflows/ci.yml` execute `go test ./...` et `go build ./...` sur Go 1.21, 1.22, 1.23, 1.24 et 1.25 sans modifier la directive `go 1.21.0`.

- [x] Faire un audit de dependances avant la premiere release
  - Domaine: securite, supply chain
  - Pourquoi: Helix expose HTTP, JWT, YAML, GORM, Prometheus et OpenTelemetry; une dependance vulnerable peut impacter directement les utilisateurs.
  - Action: ajouter `govulncheck ./...` en verification locale et CI, puis documenter la procedure de mise a jour des dependances.
  - Validation: `govulncheck` passe ou chaque finding a une decision documentee.
  - Preuve: `.github/workflows/ci.yml` execute `govulncheck ./...` et `docs/reference/release.md` documente la procedure de triage des findings avant publication.

- [x] Valider le workflow de release de bout en bout
  - Domaine: release
  - Pourquoi: GoReleaser est configure en draft sans binaire a publier; il faut confirmer que les tags `v*` produisent bien la release attendue pour un module Go.
  - Action: lancer une release seche ou sur tag de pre-release, verifier changelog, draft GitHub, checks CI et instructions d'installation.
  - Validation: une checklist de release existe et a ete executee au moins une fois.
  - Preuve: `docs/reference/release.md` fournit la checklist dry-run GoReleaser, tag de pre-release, verification du draft GitHub, checks CI et commandes d'installation.

- [ ] Clarifier le contrat de stabilite de l'API publique
  - Domaine: gouvernance, adoption
  - Pourquoi: les utilisateurs doivent savoir ce qui est stable, experimental ou susceptible de casser avant v1.
  - Action: ajouter une politique courte de compatibilite semver, deprecation et breaking changes.
  - Validation: README ou docs expliquent les garanties pour `core`, `web`, `data`, `config`, `starter`, `security`, `scheduler`, `cli` et `testutil`.

- [ ] Auditer les artefacts versionnes avant publication
  - Domaine: release, hygiene repo
  - Pourquoi: le depot contient beaucoup d'artefacts d'outillage, de skills agents et de fichiers generes; il faut distinguer ce qui appartient au framework public de ce qui est interne ou regenerable.
  - Action: lister les fichiers suivis par categorie, decider quoi garder, deplacer, ignorer ou documenter, sans retirer les artefacts internes utiles par accident.
  - Validation: `git ls-files` ne contient plus d'artefacts accidentels, ou chaque categorie conservee a une raison explicite.

- [ ] Creer un test smoke public de l'experience "30 minutes"
  - Domaine: DX, docs
  - Pourquoi: l'objectif d'onboarding rapide est declare, mais il faut le mesurer sur un parcours reel.
  - Action: definir un scenario CRUD minimal avec config, controller, service, repository, test et lancement HTTP.
  - Validation: un nouveau contributeur peut suivre le guide sans contexte interne et obtenir une API fonctionnelle.

## P1 - Robustesse Production

- [ ] Rendre la detection des starters independante du repertoire courant
  - Domaine: starters, runtime
  - Pourquoi: plusieurs starters inspectent `go.mod`; un binaire lance depuis un autre repertoire peut desactiver des starters silencieusement.
  - Action: remplacer les lectures CWD-dependantes par une detection robuste ou une configuration explicite quand le module source n'est pas disponible.
  - Validation: tests avec CWD different de la racine module pour web, data et scheduling.

- [ ] Gerer les echecs partiels de configuration des starters
  - Domaine: starters, lifecycle
  - Pourquoi: si un enregistrement reussit puis le suivant echoue, le container peut rester dans un etat partiellement configure.
  - Action: definir une strategie all-or-nothing, rollback, ou diagnostic explicite pour chaque starter.
  - Validation: tests d'echec force prouvant qu'aucun composant orphelin ne reste actif.

- [ ] Durcir l'orchestration entre starters et mode wire
  - Domaine: DI, codegen
  - Pourquoi: les composants enregistres par starters et par wire peuvent se chevaucher sans coordination claire.
  - Action: definir l'ordre, les conflits autorises et les erreurs attendues quand deux registrations ciblent le meme type.
  - Validation: tests couvrant conflit direct, interface partagee et priorite explicite.

- [ ] Exposer le contexte HTTP aux handlers Helix
  - Domaine: web, observabilite
  - Pourquoi: health checks, tracing et handlers ne peuvent pas toujours propager annulation, deadlines et child spans.
  - Action: ajouter une API stable pour recuperer le `context.Context` de la requete depuis `web.Context`.
  - Validation: un handler cree un child span et respecte une annulation client en test.

- [ ] Marquer les spans OpenTelemetry en erreur quand un handler echoue
  - Domaine: observabilite
  - Pourquoi: les traces peuvent apparaitre OK meme quand une requete retourne une erreur.
  - Action: enregistrer l'erreur et le status OpenTelemetry dans le middleware tracing.
  - Validation: test avec handler en erreur verifiant `RecordError` et status error.

- [ ] Ajouter une option TLS pour les exporters OTLP
  - Domaine: observabilite, production
  - Pourquoi: les exporters OTLP/Jaeger utilisent une configuration insecure; ce n'est pas adapte a tous les environnements production.
  - Action: ajouter des options de config pour insecure, endpoint, headers et TLS.
  - Validation: tests de resolution de config et documentation d'exemple.

- [ ] Corriger les limites connues du cache interceptor
  - Domaine: web, performance
  - Pourquoi: le cache peut subir un cache stampede et ne fait pas d'eviction proactive par taille.
  - Action: ajouter single-flight optionnel, taille maximale et sweep periodique.
  - Validation: tests concurrents cold key, eviction TTL et eviction taille.

- [ ] Durcir les guards globaux contre les valeurs nil et chemins anormaux
  - Domaine: web, securite
  - Pourquoi: un guard nil ou un chemin avec double slash peut produire un panic ou un matching inattendu.
  - Action: valider les guards a l'enregistrement et normaliser les chemins avant matching.
  - Validation: tests guard nil, `//`, slash final et patterns wildcard.

- [ ] Ameliorer le contrat des migrations DB
  - Domaine: CLI, data
  - Pourquoi: les migrations sont centrees SQLite et certains cas crash, lock ou imports du projet hote restent difficiles a diagnostiquer.
  - Action: documenter les limites, ajouter preflight CGo, TTL de lock et meilleurs messages d'erreur.
  - Validation: tests lock persistant, annulation et erreur CGo explicite.

- [ ] Preparer le support multi-dialecte data
  - Domaine: data, GORM
  - Pourquoi: plusieurs comportements sont verifies sur SQLite seulement, mais GORM vise aussi PostgreSQL/MySQL.
  - Action: ajouter une suite d'integration optionnelle PostgreSQL/MySQL pour filtres, pagination, migrations et transactions.
  - Validation: tests dialectes lances en CI optionnelle ou workflow manuel.

- [ ] Revoir les interfaces lifecycle pour l'annulation
  - Domaine: core, lifecycle
  - Pourquoi: `OnStop() error` ne permet pas d'annuler proprement un composant bloque.
  - Action: evaluer une interface additionnelle avec `context.Context` sans casser l'API existante.
  - Validation: composant bloque arrete sans goroutine abandonnee dans un test cible.

## P2 - Experience Developpeur et Surface Fonctionnelle

- [ ] Ajouter support PATCH par convention
  - Domaine: web
  - Pourquoi: les APIs REST utilisent souvent PATCH pour les mises a jour partielles; seule la convention Update/PUT est couverte.
  - Action: definir une convention `Patch` ou une directive officielle, puis documenter les regles de binding.
  - Validation: controller test avec `PATCH /resources/:id`.

- [ ] Permettre un prefixe de route explicite par controller
  - Domaine: web, DX
  - Pourquoi: versioning `/v1`, ressources imbriquees et pluralisation irreguliere ne sont pas bien servis par la convention automatique seule.
  - Action: ajouter une API `RoutePrefix()` ou option equivalente.
  - Validation: tests prefixe `/v1/users` et route imbriquee.

- [ ] Retourner plusieurs erreurs de validation
  - Domaine: web, API UX
  - Pourquoi: aujourd'hui les clients peuvent devoir corriger les champs un par un.
  - Action: etendre `ErrorResponse` ou ajouter un format compatible pour liste d'erreurs.
  - Validation: body invalide avec plusieurs champs retourne toutes les erreurs attendues.

- [ ] Supporter les query params listes et floats
  - Domaine: web, binding
  - Pourquoi: les filtres d'API courants utilisent `ids=1&ids=2`, `tags=a,b` ou des valeurs decimales.
  - Action: definir le contrat pour slices, arrays, floats et erreurs de parsing.
  - Validation: tests table-driven pour query params multi-valeurs.

- [ ] Offrir un mode JSON lenient documente
  - Domaine: web, compatibilite clients
  - Pourquoi: rejeter les champs inconnus est strict et utile, mais peut bloquer des clients forward-compatible.
  - Action: stabiliser le tag ou l'option permettant d'accepter les champs inconnus.
  - Validation: docs et tests strict/lenient.

- [ ] Generer une specification OpenAPI depuis les controllers
  - Domaine: CLI, ecosysteme
  - Pourquoi: les utilisateurs auront besoin de documentation API, clients SDK et validation contractuelle.
  - Action: ajouter une commande ou option `helix generate openapi`.
  - Validation: exemple CRUD produit un fichier OpenAPI valide.

- [ ] Ajouter des benchmarks officiels
  - Domaine: performance
  - Pourquoi: les objectifs startup et latence sont declares, mais pas encore visibles comme garde-fous continus.
  - Action: benchmarks DI startup, route health, binding JSON, cache interceptor et repository simple.
  - Validation: `go test -bench` publie des chiffres comparables dans la docs ou CI manuelle.

- [ ] Documenter les limites de production par package
  - Domaine: docs, adoption
  - Pourquoi: les utilisateurs doivent connaitre les compromis actuels sans lire les notes internes.
  - Action: ajouter une page "Production readiness" avec limites connues et mitigations.
  - Validation: page referencee depuis README et guides deploy.

- [ ] Ajouter des templates plus proches de vrais projets
  - Domaine: CLI, DX
  - Pourquoi: les scaffolds simples compilent, mais ne couvrent pas encore auth, DB, config par environnement et tests complets ensemble.
  - Action: ajouter templates `api`, `secured-api`, `gorm-api`.
  - Validation: chaque template compile, lance ses tests et demarre localement.

- [ ] Ameliorer les diagnostics du CLI pour flags et arguments
  - Domaine: CLI
  - Pourquoi: les flags places apres certains arguments peuvent produire des erreurs peu intuitives.
  - Action: uniformiser le parsing ou documenter explicitement l'ordre accepte.
  - Validation: tests pour flags avant/apres nom positionnel selon le contrat choisi.

- [ ] Ajouter une strategie de logs injectables
  - Domaine: observabilite, testabilite
  - Pourquoi: l'usage de `slog.Default()` simplifie le demarrage mais limite l'isolation en test et multi-app.
  - Action: permettre un logger applicatif explicite sans casser le mode zero-config.
  - Validation: deux apps Helix dans le meme process peuvent utiliser des loggers differents.

## P3 - Ecosysteme et Long Terme

- [ ] Adapter Ent pour la couche data
  - Domaine: data, ecosysteme
  - Pourquoi: l'architecture prevoit plusieurs adaptateurs; GORM seul limite l'audience.
  - Action: definir le contrat minimal Ent pour repository, transactions et generation.
  - Validation: exemple Ent avec CRUD et tests integration.

- [ ] Adapter sqlc pour la couche data
  - Domaine: data, performance
  - Pourquoi: certains utilisateurs Go preferent SQL explicite et generation type-safe.
  - Action: proposer une integration qui garde les abstractions Helix sans cacher sqlc.
  - Validation: exemple sqlc compile et passe les tests.

- [ ] Evaluer gRPC et WebSocket
  - Domaine: web, futur
  - Pourquoi: l'architecture les mentionne hors perimetre initial, mais ce sont des besoins backend frequents.
  - Action: produire une note de design separee avant toute implementation.
  - Validation: decision documentee: supporter, differer ou exclure.

- [ ] Ajouter des verrous distribues pour scheduling
  - Domaine: scheduler, production
  - Pourquoi: les jobs cron en multi-instance peuvent s'executer plusieurs fois.
  - Action: definir une interface de lock distribue optionnelle.
  - Validation: test avec backend fake prouvant qu'une seule instance execute le job.

- [ ] Construire un exemple deploiement complet
  - Domaine: docs, adoption
  - Pourquoi: les utilisateurs ont besoin d'un chemin production concret.
  - Action: fournir Dockerfile, healthcheck, config env, migration et observabilite pour une API exemple.
  - Validation: `docker build` et run local exposes `/actuator/health`.

- [ ] Definir une gouvernance contributeur
  - Domaine: projet, communaute
  - Pourquoi: si Helix devient public, les issues, PRs, security reports et releases doivent avoir un cadre clair.
  - Action: ajouter issue templates, security policy, contribution flow et labels.
  - Validation: un contributeur externe sait comment reporter un bug, proposer une feature et signaler une faille.

## Nettoyage Continu

- [ ] Garder `deferred-work.md` comme source detaillee et synchroniser cette roadmap quand une dette devient prioritaire.
- [ ] Transformer chaque tache P0/P1 acceptee en fiche de travail dediee avec criteres d'acceptation et validation.
- [ ] Retirer de ce fichier les taches terminees pour eviter une roadmap morte.
