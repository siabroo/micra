// Two run modes from one binary, no cobra:
//
//	server                — long-running gRPC service
//	cron-stale-rotate     — one-shot DB cleanup
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"google.golang.org/grpc"

	"github.com/siabroo/micra/adapters/loggerslog"
	"github.com/siabroo/micra/components/grpcserver"
	micrapgxpool "github.com/siabroo/micra/components/pgxpool"
	"github.com/siabroo/micra/core"
)

const dsn = "postgres://user:pass@localhost:5432/db?sslmode=disable"

func main() {
	rawLog := loggerslog.New(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	mode := "server"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	app := core.New(
		core.WithName("example-multi-mode"),
		core.WithLogger(rawLog),
	)
	pool := micrapgxpool.New(micrapgxpool.WithDSN(dsn))
	if err := app.Register(pool); err != nil {
		exit(1, err)
	}

	switch mode {
	case "server":
		if err := app.Register(grpcserver.New(
			grpcserver.WithAddr(":50500"),
			grpcserver.WithRegister(func(s *grpc.Server) { _ = s }),
		)); err != nil {
			exit(1, err)
		}
		exit(code(app.Run(context.Background())), nil)

	case "cron-stale-rotate":
		exit(code(app.RunOnce(context.Background(), func(ctx context.Context) error {
			log := core.LoggerFrom(ctx)
			log.Info("rotating stale tokens")
			// e.g. pool.DB().Exec(ctx, "UPDATE ... WHERE ...")
			return nil
		})), nil)

	default:
		fmt.Fprintln(os.Stderr, "unknown mode:", mode)
		os.Exit(2)
	}
}

func exit(c int, err error) {
	if err != nil {
		slog.Error("fatal", "error", err)
	}
	os.Exit(c)
}

func code(err error) int {
	if err != nil {
		slog.Error("app", "error", err)
		return 1
	}
	return 0
}
