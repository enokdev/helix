# Securite, observabilite et scheduling

Ce guide detaille les modules transversaux de Helix pour les applications de production.

## Objectif

Comprendre comment securiser, monitorer et planifier des traitements dans une application Helix.

## Sections prevues

- Configuration JWT
- RBAC et guards declaratifs
- `helix.SecurityConfigurer`
- Endpoints `/actuator/health`, `/actuator/metrics` et `/actuator/info`
- Logging structure avec `slog`
- Tracing OpenTelemetry
- Directive `//helix:scheduled` *(generation de code uniquement — fonctionnalite a venir)*

## References rapides

- Package `security`
- Package `observability`
- Package `scheduler`
