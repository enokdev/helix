# Lacunes P0 Remaining Release Readiness Design

## Contexte

`lacunes.md` contient sept travaux P0 pour la preparation release et adoption. Le premier item, verification d'installation externe depuis un projet neuf, est deja termine et prouve par `scripts/smoke_external_install.sh`.

Ce cycle reprend uniquement les P0 restants. Les items P1, P2, P3 et Nettoyage Continu restent hors perimetre.

## Objectif

Fermer autant de P0 restants que possible avec des preuves versionnees, sans melanger des changements runtime non necessaires. Une case de `lacunes.md` passe a `[x]` seulement quand le depot contient la procedure, le workflow, le test, le rapport ou la documentation qui satisfait l'action et la validation de l'item.

## P0 Cibles

Inclus dans ce cycle :

- Tester la compatibilite Go sur plusieurs versions supportees.
- Faire un audit de dependances avant la premiere release.
- Valider le workflow de release de bout en bout.
- Clarifier le contrat de stabilite de l'API publique.
- Auditer les artefacts versionnes avant publication.
- Creer un test smoke public de l'experience "30 minutes".

Le smoke "30 minutes" est traite apres les P0 CI, securite, release et gouvernance, car il peut toucher exemples, docs et tests de parcours utilisateur.

## Approche Retenue

Approche par batch P0 independant :

1. Traiter en parallele les P0 qui touchent des surfaces separees : CI Go, audit dependances, checklist release, stabilite API et audit des artefacts.
2. Mettre a jour `lacunes.md` pour chaque item uniquement apres preuve.
3. Inspecter le diff pour verifier que les changements restent dans les fichiers attendus.
4. Traiter le smoke "30 minutes" comme dernier item du batch si les premiers changements restent reviewables ; sinon produire une spec ou un plan separe pour ce P0.

Cette approche avance plus vite qu'une execution strictement sequencee, tout en gardant des criteres de completion stricts.

## Design Par Item

### Compatibilite Go

Modifier la CI existante pour executer `go test ./...` et `go build ./...` sur une matrice Go `1.21`, `1.22`, `1.23`, `1.24` et `1.25`. Le lint reste sur une seule version pour eviter de multiplier un controle qui ne valide pas la compatibilite runtime.

Completion : le workflow versionne contient la matrice et garde la directive module `go 1.21.0` intacte.

### Audit Dependances

Ajouter `govulncheck ./...` comme verification reproductible. La forme preferee est un workflow CI dedie ou un job distinct, afin que les vulnerabilites soient visibles sans etre confondues avec les echecs unitaires.

Ajouter une courte procedure mainteneur dans une documentation release ou securite si le workflow ne suffit pas a expliquer quoi faire quand un finding apparait.

Completion : `govulncheck` est lance par CI ou documente par une procedure versionnee, et une validation locale est tentee pendant l'implementation.

### Workflow Release

Ajouter une checklist release versionnee qui couvre dry-run GoReleaser, tag de pre-release, changelog, draft GitHub, verification CI et commandes d'installation. Ne pas lancer de vraie release GitHub dans ce cycle.

Completion : la checklist existe, reference le workflow `.github/workflows/release.yml`, et decrit comment executer une validation seche avant publication.

### Stabilite API Publique

Ajouter une politique courte de compatibilite avant v1 : garanties semver, surface stable ou experimentale par package, deprecation, breaking changes et niveau de support attendu.

Cette politique doit etre publique, sans terminologie de planning interne, et referencee depuis le README ou l'index docs.

Completion : README ou docs expliquent explicitement les garanties pour `core`, `web`, `data`, `config`, `starter`, `security`, `scheduler`, `cli` et `testutil`.

### Audit Artefacts Versionnes

Produire un audit versionne des fichiers suivis par categorie : source framework, docs publiques, exemples, CI/release, scripts mainteneur, artefacts internes utiles, artefacts accidentels ou regenerables.

Nettoyer seulement les artefacts manifestement accidentels et sans valeur de conservation. Les fichiers internes utiles peuvent rester si leur raison est explicite.

Completion : un rapport ou une procedure versionnee explique les categories conservees et les suppressions eventuelles.

### Smoke Public "30 Minutes"

Definir un parcours CRUD minimal reproductible depuis la documentation publique : installation, creation d'une app, config, controller, service, repository, test et lancement HTTP.

Le premier choix est d'ajouter un script ou test de smoke qui suit un guide public existant, puis de completer la doc si le guide ne suffit pas. Si ce travail devient trop large, il sortira dans un second cycle P0 dedie.

Completion : un contributeur peut suivre une procedure versionnee et obtenir une API fonctionnelle sans contexte interne.

## Parallelisation

Les travaux peuvent etre menes en parallele par surfaces :

- CI : matrice Go et `govulncheck`.
- Docs release/gouvernance : checklist release et stabilite API.
- Hygiene repo : audit des artefacts suivis.
- DX smoke : parcours "30 minutes", seulement apres les premiers diffs ou dans un cycle separe.

Les edits concurrents ne doivent pas modifier le meme fichier sans coordination. `lacunes.md` est mis a jour en dernier, apres consolidation des preuves.

## Validation

Validation minimale du cycle :

- `git diff` inspecte pour verifier les fichiers touches.
- Syntaxe YAML des workflows inspectee visuellement et, si disponible localement, validee par l'outillage repo.
- `/Users/yacoubakone/.govm/go/bin/go test ./...` execute si des changements Go ou scripts de smoke l'exigent.
- `govulncheck ./...` execute localement si l'outil est disponible ou installable dans l'environnement.
- Les items coches dans `lacunes.md` contiennent une preuve courte sous l'item.

## Risques et Limites

- Les workflows GitHub Actions et GoReleaser ne peuvent pas etre entierement prouves localement sans environnement GitHub. Le cycle doit donc distinguer configuration versionnee, dry-run local et validation distante.
- `govulncheck` peut necessiter un telechargement reseau. Si le reseau est bloque, la procedure reste versionnee mais l'item ne doit etre coche que si une validation suffisante est obtenue.
- L'audit des artefacts peut devenir destructif. Toute suppression doit etre limitee aux fichiers clairement accidentels et verifiee par `git diff`.
- Le smoke "30 minutes" peut depasser un batch reviewable. Dans ce cas, les autres P0 peuvent etre termines et ce smoke restera ouvert avec un plan separe.
