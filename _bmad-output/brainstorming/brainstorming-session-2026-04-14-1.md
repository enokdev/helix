---
stepsCompleted: [1, 2]
inputDocuments: [prd.md]
session_topic: 'Design et stratégie du framework Go Helix'
session_goals: 'Explorer des angles non couverts par le PRD, challenger les décisions d architecture, générer des idées sur la DX, l adoption et les fonctionnalités différenciantes'
selected_approach: 'ai-recommended'
techniques_used: ['Assumption Reversal', 'Cross-Pollination', 'Alien Anthropologist']
ideas_generated: []
context_file: 'prd.md'
---

# Brainstorming Session — Helix

**Facilitateur :** Claude
**Utilisateur :** Enokdev
**Date :** 2026-04-14

## Session Overview

**Sujet :** Design et stratégie du framework Go Helix — un framework backend Go complet et opinionné, positionné comme l'équivalent idiomatique de Spring Boot pour Go.

**Objectifs :** Explorer des angles non couverts par le PRD, challenger les décisions d'architecture avant de coder, générer des idées sur la DX, l'adoption, et les fonctionnalités différenciantes.

**Contexte :** PRD Helix v1.0 disponible — framework en phase pré-code.

## Sélection des techniques

**Approche :** AI-Recommended

- **Assumption Reversal** : Déconstruire les hypothèses fondamentales de Helix avant qu'elles ne soient gravées dans le code
- **Cross-Pollination** : Piller les patterns de Rails, Phoenix, NestJS, Actix pour enrichir le design
- **Alien Anthropologist** : Regarder l'écosystème Go avec des yeux naïfs pour trouver les frictions DX invisibles

---

## Idées générées

### Assumption Reversal

**[Assumption #1]** : *"La cible principale de Helix est le développeur Go expérimenté"*
_Concept_ : Faux. La vraie cible est le développeur Spring Boot qui migre vers Go — quelqu'un qui connaît Spring intimement mais trouve l'écosystème Go fragmenté et sans repères. La documentation doit parler le langage Spring, les concepts doivent mapper 1-pour-1 avec Spring Boot.
_Novelty_ : Ce repositionnement change toute la DX — zéro courbe d'apprentissage conceptuelle pour un développeur Spring.

**[Assumption #2]** : *"La reflection DI est une option avancée, la compile-time est le défaut"*
_Concept_ : À inverser complètement. Pour la cible Spring-migrant, la reflection doit être le mode par défaut — c'est ce qui donne la sensation "ça marche tout seul". Le mode compile-time devient l'option avancée pour les équipes Go-native qui optimisent.
_Novelty_ : Aligne Helix sur l'expérience Spring réelle plutôt que sur l'idéologie Go-native.

**[Assumption #3]** : *"L'API HTTP de Helix doit exposer Fiber directement"*
_Concept_ : Non — Fiber est un détail d'implémentation. L'API publique est déclarative via struct tags. Le développeur Spring retrouve @GetMapping sous forme de tags Go. Fiber tourne en dessous mais reste invisible.
_Novelty_ : Premier framework Go où on ne voit jamais l'objet `app` ou `router` dans le code métier.

**[Assumption #4]** : *"L'utilisateur doit construire et gérer le container explicitement"*
_Concept_ : Non — `helix.Run(App{})` est le seul point d'entrée. Helix auto-scanne, auto-wire, auto-démarre. Le container est un détail interne. Une ligne pour tout démarrer comme Spring Boot.
_Novelty_ : Élimine toute la verbosité de wiring manuel.

**[Assumption #5]** : *"Il n'existe pas d'équivalent Go des annotations Spring sur les classes"*
_Concept_ : Les structs embarqués (`helix.Service`, `helix.Controller`, `helix.Repository`) jouent exactement ce rôle. Embed = déclaration de rôle. Tags de champ = configuration des dépendances. Mapping Spring → Go 1-pour-1.
_Novelty_ : Un développeur Spring lit `helix.Service` et comprend immédiatement.

**[Assumption #6]** : *"Helix doit inventer ses propres conventions de configuration"*
_Concept_ : Non — copier exactement `application.yaml` + profils Spring (`HELIX_PROFILES_ACTIVE`). La migration Spring → Helix doit être un copier-coller de config, pas une réécriture.
_Novelty_ : Zéro friction de migration sur la configuration.

**[Assumption #7]** : *"Les tests Helix sont des tests Go classiques avec injection manuelle"*
_Concept_ : Non — `helix.NewTestApp()` + `helix.MockBean[T]()` donnent l'expérience `@SpringBootTest` + `@MockBean`. Le container complet démarre en test, les mocks sont auto-injectés.
_Novelty_ : Premier framework Go où le test d'intégration est aussi simple que le test unitaire.

**[Assumption #8]** : *"La gestion des transactions en Go doit rester manuelle"*
_Concept_ : Non — `transactional:"true"` sur une méthode, et Helix gère begin/commit/rollback via AOP au démarrage. Rollback automatique si la méthode retourne une erreur.
_Novelty_ : Premier framework Go avec AOP transactionnel déclaratif.

**[Assumption #9]** : *"La génération automatique de requêtes Spring Data est impossible en Go"*
_Concept_ : Possible via code generation + struct tags. Interface avec méthodes taguées `query:"auto"` — `helix generate` analyse les noms de méthodes et génère le SQL correspondant à la compilation.
_Novelty_ : Spring Data en Go — personne n'a encore fait ça. Feature différenciante majeure.

---

### Cross-Pollination

**[Idea #10]** : *"RESTful routing automatique par convention de nommage (Rails)"*
_Concept_ : `Index/Show/Create/Update/Delete` sur un Controller génèrent automatiquement les 5 routes REST CRUD sans configuration. Les tags `route:""` restent disponibles pour les cas custom.
_Novelty_ : Aucun framework Go ne fait ça aujourd'hui.

**[Idea #11]** : *"Migrations DB natives dans le CLI Helix (Rails)"*
_Concept_ : `helix db migrate` intégré dans le CLI — pas de dépendance externe. Fichiers versionnés en Go pur dans `db/migrations/`, rollback natif, status visible.
_Novelty_ : Aucun framework Go actuel n'intègre les migrations dans son CLI.

**[Idea #12]** : *"Contexts comme frontières de domaine explicites (Phoenix)"*
_Concept_ : Helix encourage l'organisation en Contexts (packages domaine avec API publique explicite) plutôt qu'en couches techniques plates. Le generator `helix generate context accounts` crée la structure complète.
_Novelty_ : Introduit la pensée DDD dans un framework Go naturellement.

**[Idea #13]** : *"Guards et Interceptors déclaratifs par tags sur les handlers (NestJS)"*
_Concept_ : Sécurité, cache, logging, rate-limiting déclarés directement sur les méthodes via tags. Composables, ordonnés, visibles sans ouvrir le code du handler.
_Novelty_ : Plus expressif que @PreAuthorize Spring Security ET plus lisible que les middlewares Fiber chaînés.

**[Idea #14]** : *"Extracteurs typés automatiques — params, body, headers par signature de méthode (Actix)"*
_Concept_ : Les paramètres de la méthode handler déclarent ce dont ils ont besoin — Helix parse, valide, et injecte automatiquement. Zéro `ctx.BodyParser()` manuel.
_Novelty_ : Élimine 80% du boilerplate de handler Go typique.

**[Idea #15]** : *"Mapping automatique code retour → status HTTP (Actix)"*
_Concept_ : Retourner `(*User, error)` depuis un handler = Helix sérialise automatiquement en JSON 200/201, ou en erreur JSON structurée 400/500. Le handler devient du Go pur testable sans HTTP.
_Novelty_ : Réduit le code de sérialisation à zéro dans les handlers.

---

### Alien Anthropologist

**[Idea #16]** : *"Helix comme réponse au paradoxe du choix de l'écosystème Go"*
_Concept_ : Le message marketing principal n'est pas "performances" ou "DI" — c'est "zéro décision d'infrastructure avant de coder". Se positionner explicitement contre la fragmentation de l'écosystème Go.
_Novelty_ : Positionnement identique à celui de Spring Boot contre Java EE dans les années 2000.

**[Idea #17]** : *"Error handling centralisé à la @ExceptionHandler Spring"*
_Concept_ : Les erreurs remontent automatiquement jusqu'à l'ErrorHandler enregistré via tags `handles:"ValidationError"`. Le handler ne gère que le happy path. Zéro `if err != nil` dans les handlers.
_Novelty_ : Réduit le bruit error-handling de Go à zéro dans la couche HTTP.

**[Idea #18]** : *"Scheduled tasks déclaratives via tags"*
_Concept_ : `scheduled:"0 0 * * *"` sur une méthode de Service = cron job automatiquement enregistré au démarrage. Sans bibliothèque externe, sans goroutine manuelle.
_Novelty_ : Cron jobs déclaratifs comme @Scheduled Spring — inexistant dans l'écosystème Go actuel.

