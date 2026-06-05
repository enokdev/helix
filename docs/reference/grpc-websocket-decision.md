# gRPC and WebSocket Decision

Helix is built on Fiber and currently optimizes for REST APIs first. The framework's value is fast CRUD and HTTP application setup on top of HTTP/1.1, with HTTP/2 available when the deployment stack enables it. Its public web model today is controller-oriented, JSON-first, and centered on a single HTTP server lifecycle. Any new transport must justify its complexity against that baseline.

## gRPC evaluation

gRPC fits Go well in isolation: `grpc-go` is mature, contracts are explicit, streaming is strong, and HTTP/2 is a natural transport. It does **not** fit Helix's current Fiber-centered architecture well. A real first-party integration would require a parallel server stack outside the normal Fiber request pipeline, plus protobuf generation, interceptors, auth, observability, examples, CLI support, and deployment guidance. That would introduce two distinct programming models in the framework.

The audience cost/benefit is also unfavorable right now. Helix targets teams that primarily want REST APIs with Spring Boot-style ergonomics. That audience is much larger than the subset that needs native framework-level gRPC. Building gRPC support now would slow delivery on the core REST, starter, and observability roadmap.

**Decision: defer to external integration guide.** Helix will document how to run `grpc-go` alongside Helix or as a separate service while sharing container-managed business components where practical.

## WebSocket evaluation

WebSocket is a better fit. Fiber already has WebSocket support, and the integration path is incremental: authenticate on an HTTP route, upgrade the connection, resolve handlers from the container, and close connections during lifecycle shutdown. This extends the existing web package instead of creating a second server model.

**Decision: support in future milestone.** WebSocket support belongs in Helix once the REST surface is stable enough to absorb a focused real-time extension.

## Recommended actions

- Publish a gRPC integration guide with a side-by-side Helix + `grpc-go` example.
- Define a minimal WebSocket API for route registration, auth-before-upgrade, and graceful shutdown.
- Add one example app for notifications or live dashboards over WebSocket.
