package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v4"
	"github.com/operate-first/curator-operator/internal"
	"github.com/operate-first/curator-operator/internal/signals"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	defaultServerPort = ":8080"
	defaultTimeout    = time.Second * 10
)

var (
	reportLog = ctrl.Log.WithName("report")
)

func newCmd() *cobra.Command {
	var (
		serverPort string
	)
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts the http server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runServer(context.Background(), reportLog, serverPort); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serverPort, "port", defaultServerPort, "configures the default port for the HTTP server")

	return cmd
}

func main() {
	opts := zap.Options{
		Development: true,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
	cmd := newCmd()
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServer(ctx context.Context, logger logr.Logger, httpServerPort string) error {
	var psqlconn = os.Getenv("POSTGRES_URI")
	conn, connErr := pgx.Connect(ctx, psqlconn)
	if connErr != nil {
		logger.Error(connErr, "failed to open a database connection to postgres")
		return connErr
	}

	defer conn.Close(ctx)

	internal.NewHTTPServer(ctx, httpServerPort, conn, logger)

	server := http.Server{
		Addr: httpServerPort,
	}

	logger.Info("starting http server on port" + httpServerPort)

	go startServer(&server, logger)

	err := shutdownServer(ctx, defaultTimeout, &server, logger)
	if err != nil {
		return err
	}
	return nil
}

func startServer(server *http.Server, logger logr.Logger) {
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error(err, "failed to listen on http server")
	}
	logger.Info("http server stopped gracefully")
}

func shutdownServer(ctx context.Context, timeout time.Duration, server *http.Server, logger logr.Logger) error {
	stopCh, closeCh := signals.CreateChannel()
	defer closeCh()
	sigReceived := <-stopCh
	logger.Info("received shutdown signal", "signal", sigReceived)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error(err, "failed to graceful shutdown server")
		return err
	}
	logger.Info("http server shutdowned")

	return nil
}
