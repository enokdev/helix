# Lacunes P0 Release Readiness Design

## Contexte

`lacunes.md` liste les travaux post-v0.1.0 encore ouverts. Le premier cycle de travail se limite aux items `P0 - Bloquants Release et Adoption` pour garder un changement reviewable et eviter de melanger des sujets independants.

Le fichier est actuellement non suivi par git. Il doit etre traite comme une roadmap actionnable locale : une case ne passe a terminee que si le depot contient une preuve suffisante ou si une validation reproductible passe pendant le cycle.

## Objectif

Mettre `lacunes.md` a jour pour les taches P0 deja terminees, puis implementer la premiere tache P0 encore ouverte avec une validation locale claire.

## Perimetre

Inclus :

- Lire les sept items P0 de `lacunes.md`.
- Comparer chaque item aux workflows CI, docs, scripts, exemples et tests existants.
- Marquer en `[x]` uniquement les items P0 prouvablement termines.
- Pour le premier item P0 encore ouvert, ajouter l'implementation minimale qui satisfait l'action et la validation de `lacunes.md`.
- Executer les commandes de validation pertinentes.

Exclus :

- Les items P1, P2, P3 et Nettoyage Continu.
- Les changements de gouvernance GitHub externes qui exigent un acces reseau ou une publication.
- Le lancement d'une vraie release GitHub.
- Les refactorings non necessaires a l'item P0 choisi.

## Regles de Completion

Une tache P0 peut etre marquee terminee si au moins un des criteres suivants est vrai :

- Un test, workflow, script ou document versionne couvre explicitement l'action et la validation demandees.
- Une commande locale reproductible passe et laisse une procedure versionnee.
- L'item est deja satisfait par une implementation existante, avec fichiers et tests identifiables.

Une tache P0 reste ouverte si la preuve depend uniquement d'une intention, d'une story archivee, ou d'une verification manuelle non documentee.

## Approche Retenue

Approche sequencee :

1. Auditer les P0 contre le repo.
2. Mettre a jour `lacunes.md` pour les P0 deja satisfaites.
3. Choisir le premier P0 encore ouvert dans l'ordre du fichier.
4. Implementer le plus petit artefact durable necessaire.
5. Valider localement avec le binaire Go govm du projet quand des tests Go sont requis.

Cette approche garde la revue focalisee et evite de transformer une roadmap en gros changement multi-domaines.

## Validation

La validation finale doit inclure :

- `git diff` inspecte pour verifier que seuls les fichiers pertinents changent.
- Une commande de test ou de verification correspondant a l'item P0 implemente.
- Si la validation reseau est necessaire mais bloquee par l'environnement, documenter le blocage et laisser une procedure executable.

## Risques

- Certains items P0 touchent CI ou release et peuvent necessiter GitHub Actions ou reseau. Dans ce cas, le cycle doit produire une configuration ou une checklist versionnee plutot que pretendre avoir execute l'etape distante.
- `lacunes.md` etant non suivi initialement, l'ajouter au commit peut exposer toute la roadmap. C'est acceptable pour ce cycle puisque la demande utilisateur porte directement sur ce fichier.
