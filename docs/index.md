---
layout: home

hero:
  name: "Helix"
  text: "High-Performance Go Framework"
  tagline: Spring Boot ergonomics with Go's speed. Production-ready by default.
  image:
    src: /logo.svg
    alt: Helix
  actions:
    - theme: brand
      text: Get Started
      link: /guide/introduction
    - theme: alt
      text: Documentation
      link: /guide/quick-start
    - theme: alt
      text: GitHub
      link: https://github.com/enokdev/helix

---

<div class="vp-doc" style="max-width: 1152px; margin: 0 auto; padding: 4rem 1.5rem;">

<h2 style="text-align: center; font-size: 2.75rem; font-weight: 800; margin-bottom: 3.5rem; letter-spacing: -0.02em;">
  The Backend Framework for Go Power Users.
</h2>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 1.5rem; margin-bottom: 5rem;">
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21 16-4 4-4-4"/><path d="M17 20V4"/><path d="m3 8 4-4 4 4"/><path d="M7 4v16"/></svg>'
    title="Advanced DI / IoC" 
    details="Type-safe dependency injection with zero boilerplate. Automatic lifecycle management and circular dependency detection."
    link="/guide/dependency-injection"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
    title="Convention Routing" 
    details="RESTful API design by convention. Automatic route discovery based on controller method names and smart defaults."
    link="/guide/web"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.1a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.1a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>'
    title="Starter Ecosystem" 
    details="Auto-configuring modules for everything: Data, Web, Security, Observability. Drop in a package and it just works."
    link="/reference/starters"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>'
    title="Enterprise Security" 
    details="JWT authentication and declarative RBAC built-in. Secure your application with production-grade defaults and clean APIs."
    link="/guide/security"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 3v18h18"/><path d="m19 9-5 5-4-4-3 3"/></svg>'
    title="Deep Observability" 
    details="Native support for Prometheus, OpenTelemetry, and structured logging. Real-time application monitoring with Actuator endpoints."
    link="/guide/observability"
  />
  <FeatureCard 
    icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/><path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/></svg>'
    title="Data Persistence" 
    details="Clean Repository pattern with GORM integration. Advanced filtering, pagination, and transaction management out of the box."
    link="/guide/database"
  />
</div>

<div style="background: var(--vp-c-bg-soft); border-radius: 20px; padding: 3.5rem; border: 1px solid var(--vp-c-divider); position: relative; overflow: hidden;">
<div style="position: absolute; top: -100px; right: -100px; width: 300px; height: 300px; background: var(--helix-gradient); opacity: 0.05; filter: blur(80px); border-radius: 50%;"></div>

<h3 style="margin-top: 0; margin-bottom: 1.5rem; font-size: 1.8rem; font-weight: 700;">Minimalist Code, Maximum Power</h3>

<p style="color: var(--vp-c-text-2); margin-bottom: 2.5rem; font-size: 1.1rem; line-height: 1.6;">
  Focus on your domain logic. Helix handles the infrastructure wiring, routing, and configuration so you can ship faster.
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

<CustomCallout type="tip" title="No Magic, Just Smart Defaults">
  Helix uses Go's type system and reflection to discover your components. No complex XML, no hidden state, just clean idiomatic Go code.
</CustomCallout>

</div>

</div>
