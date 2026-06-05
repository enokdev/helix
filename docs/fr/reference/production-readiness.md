# Readiness production

Cette page documente les limites production actuelles par package et les mitigations recommandées. Elle est volontairement conservatrice : si un comportement n'est pas implémenté ou vérifié en continu, considérez-le comme une limite.

## `core`

- La DI par réflexion est le mode par défaut et convient à la plupart des services, mais le câblage à la compilation est préférable pour les démarrages sensibles à la latence. Utilisez `helix generate wire` quand la variance de startup compte.
- L'arrêt lifecycle dépend du respect de `OnStop(ctx context.Context)` par les composants. Un composant qui ignore l'annulation peut consommer tout le budget d'arrêt.
- Le container détecte les doublons et cycles de dépendances, mais ne remplace pas des frontières de packages propres.

## `config`

- La priorité de configuration est `ENV > YAML profile > application.yaml > default`. Ne vous reposez pas sur des defaults implicites pour les secrets de production.
- Les hooks de reload config sont locaux au process. La propagation distribuée reste à la charge de l'application.
- Gardez les secrets dans les variables d'environnement ou le secret store de la plateforme, pas dans les fichiers YAML.

## `web`

- L'implémentation HTTP est basée sur Fiber en interne; le code applicatif public doit utiliser `web.Context` et les interfaces publiques Helix.
- Le cache interceptor est en mémoire et local au process. Il supporte TTL, taille maximale, sweep et coalescing cold-key, mais ce n'est pas un cache distribué.
- Protégez les routes administratives comme `/actuator/metrics` avec un guard si elles sont exposées hors réseau privé.
- Les deadlines et annulations sont disponibles via `web.Context.Context()`, mais les handlers longs doivent vérifier ce contexte.

## `data` et `data/gorm`

- Les migrations SQLite sont supportées par le CLI aujourd'hui. L'exécution PostgreSQL/MySQL n'est pas encore activée, même si les tests d'intégration repository couvrent les dialectes quand les DSN sont fournis.
- Les migrations SQLite requièrent CGo car elles utilisent `github.com/mattn/go-sqlite3`.
- Configurez explicitement les limites du pool de base de données pour les charges production.
- Le repository générique est une couche de commodité; utilisez des requêtes dédiées pour les hot paths ou le SQL complexe.

## `observability`

- `/actuator/health`, `/actuator/metrics` et `/actuator/info` sont disponibles, mais la couverture health dépend des indicateurs enregistrés pour chaque dépendance externe.
- Le tracing OTLP supporte endpoint, headers, mode insecure, TLS et mTLS. Utilisez `insecure: false` avec les paramètres TLS pour les collectors production.
- La cardinalité des métriques reste sous responsabilité applicative.

## `security`

- La validation JWT exige un secret robuste ou une source de clés fiable. N'utilisez pas de secrets d'exemple en production.
- Les guards RBAC protègent les routes seulement s'ils sont enregistrés globalement ou sur la route/le controller concerné.
- La révocation de tokens et la gestion de session sont des responsabilités applicatives.

## `scheduler`

- Le scheduling cron est local au process. En déploiement multi-instance, le même job peut tourner sur chaque réplica.
- Les locks distribués ne sont pas encore implémentés. Utilisez un seul réplica scheduler ou un lock applicatif pour les jobs à effets de bord.
- Les jobs longs doivent respecter l'annulation du contexte pendant l'arrêt.

## `starter`

- Les starters auto-configurent l'infrastructure commune et annulent les registrations framework quand une étape ultérieure échoue.
- L'auto-détection dépend du contenu du module et de la config explicite. En production, préférez une config starter explicite pour l'infrastructure critique.

## `cli`

- `helix run` est une commande de développement avec hot reload. Utilisez `helix build` et exécutez le binaire produit en production.
- Les commandes nommées acceptent les flags avant ou après le nom positionnel, par exemple `helix generate module --dir ./app user` et `helix generate module user --dir ./app`.
- Les migrations compilent dans un module temporaire isolé. Gardez les imports de migrations autonomes.

## `testutil`

- Les helpers de test sont destinés aux tests applicatifs et framework, pas au wiring production.
- Les mocks remplacent les composants enregistrés dans un container de test uniquement; ils ne modifient pas l'auto-registration globale hors de ce process de test.

## Checks de release

Avant de publier une release destinée à la production, exécutez :

```bash
go test ./...
go test ./core ./observability ./web ./data/gorm -run '^$' \
  -bench 'Benchmark(ReflectResolver|ActuatorHealthRoute|BindingJSON|CacheInterceptorHit|RepositoryFindByIDSQLite)' \
  -benchmem
govulncheck ./...
```

Voir aussi [Référence performance](./performance.md), [Déploiement](./deployment.md) et [Stabilité API](./api-stability.md).
