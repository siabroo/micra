// Minimal micra example: one gRPC server, default interceptors, health
// probe. Run with: go run . then `grpc_health_probe -addr 127.0.0.1:50500`.
package main

import (
	"context"
	"log/slog"
	"os"

	"google.golang.org/grpc"

	"github.com/siabroo/micra/adapters/loggerslog"
	"github.com/siabroo/micra/components/grpcserver"
	"github.com/siabroo/micra/core"
)

func main() {
	log := loggerslog.New(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	app := core.New(
		core.WithName("example-grpc"),
		core.WithLogger(log),
	)

	if err := app.Register(grpcserver.New(
		grpcserver.WithAddr("127.0.0.1:50500"),
		grpcserver.WithRegister(func(s *grpc.Server) {
			// register your services here
			_ = s
		}),
	)); err != nil {
		log.Error("register", "error", err)
		os.Exit(1)
	}

	if err := app.Run(context.Background()); err != nil {
		log.Error("exit", "error", err)
		os.Exit(1)
	}
}
