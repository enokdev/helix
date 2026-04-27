package web

// HTTPController is the marker interface for Helix controller components.
// It is an empty interface used as a type constraint for auto-discovery —
// components registered in the DI container that satisfy this interface are
// automatically registered with the HTTP server.
//
// A struct satisfies HTTPController when it embeds helix.Controller:
//
//	type UserController struct {
//	    helix.Controller
//	}
//
// The embed provides no methods — the framework detects the helix.Controller
// field via struct reflection inside RegisterController.
type HTTPController interface{}
