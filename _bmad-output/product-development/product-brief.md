# Product Brief — Helix

**Version** : 1.0  
**Date** : 2026-04-14  
**Statut** : Draft  

---

## Problème

Les développeurs Spring Boot qui migrent vers Go se retrouvent face à un écosystème fragmenté : des frameworks HTTP légers (Gin, Echo, Fiber), des outils DI découplés (Wire, fx), aucune convention partagée, aucune solution complète. Ils doivent assembler eux-mêmes des briques disparates et réapprendre de zéro des patterns qu'ils maîtrisent déjà.

Le résultat : des projets incohérents, un onboarding difficile, et une productivité bien en deçà de ce qu'ils atteignaient avec Spring Boot.

---

## Vision

> Helix est le Spring Boot de Go — complet, opinionné, idiomatique.

Un développeur Spring Boot doit pouvoir ouvrir Helix et se sentir immédiatement chez lui. Les concepts mappent 1-pour-1. Les conventions sont identiques. La magie fonctionne.

---

## Cible principale

**Développeurs Spring Boot qui adoptent Go.**

Secondairement : équipes Go qui buildent plusieurs microservices et veulent une base cohérente entre projets.

---

## Proposition de valeur

| Pour qui | Ce que Helix offre |
|---|---|
| Dev Spring Boot migrant | Zéro réapprentissage conceptuel — les mêmes patterns, en Go |
| Équipe microservices | Une base cohérente et production-ready sans assemblage manuel |
| Lead technique | Standardisation de l'architecture entre projets sans effort |

---

## Différenciation

Helix est le **seul framework Go** qui :

1. Traite la DX Spring Boot comme exigence de premier niveau
2. Offre DI, HTTP, Data, Config, Observabilité, CLI dans un seul outil cohérent
3. Mappe explicitement ses concepts sur Spring Boot (`helix.Service` = `@Service`, `inject:"true"` = `@Autowired`, `query:"auto"` = Spring Data, etc.)
4. Démarre en une ligne : `helix.Run(App{})`

---

## Fonctionnalités clés (résumé)

- **DI par reflection** (par défaut) + mode compile-time (opt-in)
- **`helix.Run(App{})`** — point d'entrée unique, container invisible
- **Composants déclaratifs** via embeds (`helix.Service`, `helix.Controller`, `helix.Repository`)
- **Routing RESTful automatique** par convention de nommage (`Index/Show/Create/Update/Delete`)
- **HTTP déclaratif** — tags sur méthodes, extracteurs typés, return type → HTTP status auto
- **Spring Data-like** — interface + `query:"auto"` + codegen
- **`transactional:"true"`** — AOP transactionnel déclaratif
- **Profils de configuration** — `application-dev.yaml`, `HELIX_PROFILES_ACTIVE`
- **Tests** — `helix.NewTestApp()` + `helix.MockBean[T]()`
- **Guards & Interceptors** déclaratifs via tags
- **`scheduled:"cron"`** — cron jobs déclaratifs
- **Error handling centralisé** — `helix.ErrorHandler` + `handles:"ErrorType"`
- **Migrations CLI** — `helix db migrate`
- **Contexts de domaine** — `helix generate context`

---

## Positionnement marché

```
                    Complet
                       ↑
              Helix ●  |
                       |
Spécifique ←-----------+----------→ Généraliste
    (DI)    fx ● wire●|  ● chi/echo
                       |
                       ↓
                    Partiel
```

Helix occupe le quadrant "Complet + Généraliste" — aujourd'hui vide dans l'écosystème Go.

---

## Critères de succès

| KPI | Cible |
|---|---|
| Temps de démarrage | < 100ms |
| Onboarding | < 30 min pour une API CRUD complète |
| Adoption | 500 GitHub stars dans les 6 mois post-launch |
| Rétention | > 70% des projets démarrés avec Helix l'utilisent en production |
