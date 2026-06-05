# Décision gRPC et WebSocket

Helix est construit sur Fiber et vise d'abord les API REST. La promesse du framework aujourd'hui est de livrer rapidement des services HTTP idiomatiques, majoritairement JSON, sur une pile simple basée sur HTTP/1.1, avec HTTP/2 disponible selon l'environnement de déploiement. Le modèle web public repose sur les controllers, le binding HTTP et un cycle de vie de serveur unique. Tout nouveau transport doit donc prouver qu'il améliore ce socle sans le fragmenter.

## Évaluation gRPC

gRPC est un bon choix en Go pris isolément : `grpc-go` est mature, Protobuf donne des contrats forts, et le streaming est excellent. En revanche, l'adéquation avec l'architecture actuelle de Helix est faible. Un support natif exigerait une pile serveur parallèle à Fiber, avec génération Protobuf, interceptors dédiés, auth spécifique, instrumentation, exemples, commandes/outillage et documentation de déploiement. Cela reviendrait à faire coexister deux modèles web dans le framework.

Le rapport coût / audience n'est pas favorable à ce stade. Helix s'adresse d'abord aux équipes qui veulent exposer des API REST rapidement avec une ergonomie inspirée de Spring Boot. Le besoin d'un support gRPC natif existe, mais il concerne un public plus restreint que le cœur de cible actuel.

**Décision : différer vers un guide d'intégration externe.** Helix ne fournit pas de starter gRPC natif pour l'instant. La voie recommandée est d'exécuter `grpc-go` à côté de Helix, ou dans un service séparé, tout en réutilisant la logique métier lorsque c'est pertinent.

## Évaluation WebSocket

WebSocket s'intègre beaucoup mieux. Fiber dispose déjà du support nécessaire, et le chemin d'intégration est direct : auth sur une route HTTP, upgrade de connexion, handlers résolus par le container, et fermeture propre lors du shutdown. On reste dans la pile web existante, sans second modèle serveur.

**Décision : support dans un futur jalon.** WebSocket mérite un support natif ciblé, une fois la surface REST suffisamment stabilisée.

## Actions recommandées

- Rédiger un guide d'intégration gRPC avec un exemple Helix + `grpc-go`.
- Définir une API minimale WebSocket pour enregistrement des routes, auth avant upgrade et arrêt propre.
- Ajouter un exemple applicatif de notifications temps réel ou tableau de bord live.
