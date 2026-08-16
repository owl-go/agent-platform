package server

import (
	"context"
	"log/slog"

	"agent-platform/backend/internal/platformconfig"

	kratos "github.com/go-kratos/kratos/v3"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func NewAPIApp(ctx context.Context, config platformconfig.Config, logger *slog.Logger, httpServer *kratoshttp.Server) *kratos.App {
	return kratos.New(
		kratos.Context(ctx),
		kratos.Name("agent-platform-api"),
		kratos.Version("dev"),
		kratos.Logger(logger),
		kratos.StopTimeout(config.API.ShutdownTimeout.Value()),
		kratos.Server(httpServer),
	)
}
