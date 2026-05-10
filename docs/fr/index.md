---
layout: home

hero:
  name: "Helix"
  text: "Framework Go Haute Performance"
  tagline: L'ergonomie de Spring Boot avec la rapidité de Go. Prêt pour la production par défaut.
  image:
    src: /logo.svg
    alt: Helix
  actions:
    - theme: brand
      text: Démarrer
      link: /fr/guide/introduction
    - theme: alt
      text: Démarrage rapide
      link: /fr/guide/quick-start
    - theme: alt
      text: GitHub
      link: https://github.com/enokdev/helix

features:
  - icon: ↕️
    title: DI / IoC Avancé
    details: Injection de dépendances type-safe sans boilerplate. Gestion automatique du cycle de vie et détection des dépendances circulaires.
    link: /fr/guide/dependency-injection
  - icon: 🌐
    title: Routage par Convention
    details: API RESTful par convention. Découverte automatique des routes selon les noms de méthodes des contrôleurs et des défauts intelligents.
    link: /fr/guide/web
  - icon: ⚙️
    title: Écosystème de Starters
    details: Modules auto-configurants pour tout — Data, Web, Sécurité, Observabilité. Ajoutez un package et ça fonctionne.
    link: /fr/reference/starters
  - icon: 🔒
    title: Sécurité Entreprise
    details: Authentification JWT et RBAC déclaratif intégrés. Sécurisez votre application avec des défauts de qualité production et des APIs propres.
    link: /fr/guide/security
  - icon: 📈
    title: Observabilité Profonde
    details: Support natif de Prometheus, OpenTelemetry et des logs structurés. Monitoring en temps réel avec les endpoints Actuator.
    link: /fr/guide/observability
  - icon: 🗄️
    title: Persistance des Données
    details: Pattern Repository propre avec intégration GORM. Filtrage avancé, pagination et gestion des transactions inclus.
    link: /fr/guide/database
---

<div class="vp-doc home-content">

## Code Minimaliste, Puissance Maximale

Concentrez-vous sur votre logique métier. Helix gère le câblage de l'infrastructure, le routage et la configuration pour que vous puissiez livrer plus vite.

```go
type UserController struct {
    helix.Controller
}

// GET /users
func (c *UserController) Index() []string {
    return []string{"alice", "bob"}
}

func main() {
    helix.Run(helix.App{
        Components: []any{&UserController{}},
    })
}
```

::: tip Pas de Magie, Juste des Défauts Intelligents
Helix utilise le système de types de Go et la réflexion pour découvrir vos composants. Pas de XML complexe, pas d'état caché, juste du code Go idiomatique et propre.
:::

</div>

<style>
.home-content {
  max-width: 900px;
  margin: 3rem auto;
  padding: 0 1.5rem;
}
</style>
