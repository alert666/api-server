package server

import (
	"github.com/google/wire"
)

// GRPCServerProviderSet gRPC server 层的 Wire provider set
var GRPCServerProviderSet = wire.NewSet(
	NewGRPCServer,
)
