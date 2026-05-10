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
      text: Quick Start
      link: /guide/quick-start
    - theme: alt
      text: GitHub
      link: https://github.com/enokdev/helix

features:
  - icon: ↕️
    title: Advanced DI / IoC
    details: Type-safe dependency injection with zero boilerplate. Automatic lifecycle management and circular dependency detection.
    link: /guide/dependency-injection
  - icon: 🌐
    title: Convention Routing
    details: RESTful API design by convention. Automatic route discovery based on controller method names and smart defaults.
    link: /guide/web
  - icon: ⚙️
    title: Starter Ecosystem
    details: Auto-configuring modules for everything — Data, Web, Security, Observability. Drop in a package and it just works.
    link: /reference/starters
  - icon: 🔒
    title: Enterprise Security
    details: JWT authentication and declarative RBAC built-in. Secure your application with production-grade defaults and clean APIs.
    link: /guide/security
  - icon: 📈
    title: Deep Observability
    details: Native support for Prometheus, OpenTelemetry, and structured logging. Real-time monitoring with Actuator endpoints.
    link: /guide/observability
  - icon: 🗄️
    title: Data Persistence
    details: Clean Repository pattern with GORM integration. Advanced filtering, pagination, and transaction management out of the box.
    link: /guide/database
---

<div class="vp-doc home-content">

## Minimalist Code, Maximum Power

Focus on your domain logic. Helix handles the infrastructure wiring, routing, and configuration so you can ship faster.

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

::: tip No Magic, Just Smart Defaults
Helix uses Go's type system and reflection to discover your components. No complex XML, no hidden state, just clean idiomatic Go code.
:::

</div>

<style>
.home-content {
  max-width: 900px;
  margin: 3rem auto;
  padding: 0 1.5rem;
}
</style>
