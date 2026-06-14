package handler

import "github.com/google/wire"

// HandlerProviderSet gRPC handler 层的 Wire provider set
var HandlerProviderSet = wire.NewSet(
	NewTunnelHandler,
)
