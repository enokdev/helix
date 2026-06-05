# Référence performance

Helix maintient des micro-benchmarks officiels pour les surfaces qui influencent le démarrage et la latence des requêtes. Ils servent à comparer localement avant une release et à diagnostiquer les régressions; ils ne remplacent pas des tests de charge applicatifs.

## Exécuter les benchmarks

```bash
go test ./... -run '^$' -bench 'Benchmark' -benchmem
```

Pour le lot maintenu de readiness release :

```bash
go test ./core ./observability ./web ./data/gorm -run '^$' \
  -bench 'Benchmark(ReflectResolver|ActuatorHealthRoute|BindingJSON|CacheInterceptorHit|RepositoryFindByIDSQLite)' \
  -benchmem
```

## Benchmarks officiels

| Benchmark | Package | Objectif |
|-----------|---------|----------|
| `BenchmarkRun_ZeroParams` | racine | Bootstrap applicatif zero-config |
| `BenchmarkRunMinimalLifecycle` | racine | Démarrage lifecycle minimal |
| `BenchmarkReflectResolverRegisterAndResolve` | `core` | Enregistrement DI puis première résolution par réflexion |
| `BenchmarkReflectResolverResolveSingleton` | `core` | Résolution DI d'un singleton déjà chaud |
| `BenchmarkActuatorHealthRoute` | `observability` | Chemin de requête `/actuator/health` |
| `BenchmarkBindingJSON` | `web` | Binding JSON et exécution du plan de validation |
| `BenchmarkCacheInterceptorHit` | `web` | Chemin cache hot-key de l'intercepteur |
| `BenchmarkRepositoryFindByIDSQLite` | `data/gorm` | Lookup repository simple via GORM et SQLite |

## Interpréter les résultats

Comparez deux commits sur la même machine, avec la même version de Go et les mêmes flags de build. Gardez la sortie brute dans les notes de release uniquement si elle soutient une affirmation de performance.

Pour une validation production, benchmarkez Helix dans l'application cible avec middleware, base de données, réseau, TLS, logs, tracing et tailles de payload réalistes.
