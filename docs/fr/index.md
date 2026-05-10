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
      text: Documentation
      link: /fr/guide/quick-start
    - theme: alt
      text: GitHub
      link: https://github.com/enokdev/helix

---

<div class="vp-doc" style="max-width: 1152px; margin: 0 auto; padding: 4rem 1.5rem;">

<h2 style="text-align: center; font-size: 2.75rem; font-weight: 800; margin-bottom: 3.5rem; letter-spacing: -0.02em; line-height: 1.25;">
  Le Framework Backend pour les Développeurs Go Exigeants.
</h2>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 1.5rem; margin-bottom: 5rem;">
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21 16-4 4-4-4"/><path d="M17 20V4"/><path d="m3 8 4-4 4 4"/><path d="M7 4v16"/></svg>'
    title="DI / IoC Avancé" 
    details="Injection de dépendances type-safe sans boilerplate. Gestion automatique du cycle de vie et détection des dépendances circulaires."
    link="/fr/guide/dependency-injection"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
    title="Routage par Convention" 
    details="API RESTful par convention. Découverte automatique des routes selon les noms de méthodes des contrôleurs et des défauts intelligents."
    link="/fr/guide/web"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.1a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.1a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>'
    title="Écosystème de Starters" 
    details="Modules auto-configurants pour tout : Data, Web, Sécurité, Observabilité. Ajoutez un package et ça fonctionne."
    link="/fr/reference/starters"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>'
    title="Sécurité Entreprise" 
    details="Authentification JWT et RBAC déclaratif intégrés. Sécurisez votre application avec des défauts de qualité production et des APIs propres."
    link="/fr/guide/security"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 3v18h18"/><path d="m19 9-5 5-4-4-3 3"/></svg>'
    title="Observabilité Profonde" 
    details="Support natif de Prometheus, OpenTelemetry et des logs structurés. Monitoring en temps réel avec les endpoints Actuator."
    link="/fr/guide/observability"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/><path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/></svg>'
    title="Persistance des Données" 
    details="Pattern Repository propre avec intégration GORM. Filtrage avancé, pagination et gestion des transactions inclus."
    link="/fr/guide/database"
  />
</div>

<div style="background: var(--vp-c-bg-soft); border-radius: 20px; padding: 3.5rem; border: 1px solid var(--vp-c-divider); position: relative; overflow: hidden;">
<div style="position: absolute; top: -100px; right: -100px; width: 300px; height: 300px; background: var(--helix-gradient); opacity: 0.05; filter: blur(80px); border-radius: 50%;"></div>

<h3 style="margin-top: 0; margin-bottom: 1.5rem; font-size: 1.8rem; font-weight: 700;">Code Minimaliste, Puissance Maximale</h3>

<p style="color: var(--vp-c-text-2); margin-bottom: 2.5rem; font-size: 1.1rem; line-height: 1.6;">
  Concentrez-vous sur votre logique métier. Helix gère le câblage de l'infrastructure, le routage et la configuration pour que vous puissiez livrer plus vite.
</p>

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

<CustomCallout type="tip" title="Pas de Magie, Juste des Défauts Intelligents">
  Helix utilise le système de types de Go et la réflexion pour découvrir vos composants. Pas de XML complexe, pas d'état caché, juste du code Go idiomatique et propre.
</CustomCallout>

</div>

</div>
