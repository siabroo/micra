// Package core defines micra's lifecycle primitives.
//
// The two top-level types are App (the runtime) and Component (the
// interface implemented by anything that needs to be Init'd, Start'd,
// and Stop'd). This package's doc comments cover the public API surface.
//
// Typical usage:
//
//	import (
//	    "github.com/siabroo/micra/core"
//	    "github.com/siabroo/micra/adapters/loggerslog"
//	    "github.com/siabroo/micra/components/grpcserver"
//	    "github.com/siabroo/micra/components/pgxpool"
//	)
//
//	func main() {
//	    log := loggerslog.New(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
//	    app := core.New(
//	        core.WithName("my-svc"),
//	        core.WithVersion(commitSHA),
//	        core.WithLogger(log),
//	    )
//	    pool := pgxpool.New(pgxpool.WithDSN(os.Getenv("DATABASE_URL")))
//	    _ = app.Register(pool)
//	    _ = app.Register(grpcserver.New(
//	        grpcserver.WithAddr(":50051"),
//	        grpcserver.WithRegister(func(s *grpc.Server) { /* ... */ }),
//	    ))
//	    if err := app.Run(context.Background()); err != nil {
//	        log.Error("app exited", "error", err)
//	        os.Exit(1)
//	    }
//	}
package core
