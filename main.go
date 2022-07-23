/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v4"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	curatorv1alpha1 "github.com/operate-first/curator-operator/api/v1alpha1"
	"github.com/operate-first/curator-operator/controllers"
	"github.com/operate-first/curator-operator/internal"
	dbCustom "github.com/operate-first/curator-operator/internal/db"
	"github.com/operate-first/curator-operator/internal/signals"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	defaultServerPort = ":8082"
	defaultTimeout    = time.Second * 10
	dbName            = "DATABASE_NAME"
	dbHost            = "DATABASE_HOST_NAME"
	dbPort            = "PORT_NUMBER"
	dbPassword        = "DATABASE_PASSWORD"
	dbUser            = "DATABASE_USER"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(curatorv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var serverPort string
	flag.StringVar(&serverPort, "http-port", defaultServerPort, "configures the default port for the HTTP server")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f0fd8a02.operatefirst.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	port, err := strconv.Atoi(os.Getenv(dbPort))
	if err != nil {
		setupLog.Error(err, "unable to convert port value")
		os.Exit(1)
	}

	config, _ := pgx.ParseConfig("")
	config.Host = os.Getenv(dbHost)
	config.Port = uint16(port)
	config.User = os.Getenv(dbUser)
	config.Password = os.Getenv(dbPassword)
	config.Database = os.Getenv(dbName)

	db, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		setupLog.Error(err, "failed to open a database connection to postgres")
		os.Exit(1)
	} else {
		_, err := dbCustom.CreateTablesAndRoutines(context.Background(), db)
		if err != nil {
			setupLog.Error(err, "Unable to create tables in the DB")
			os.Exit(1)
		}
	}
	defer db.Close(context.Background())

	if err = (&controllers.ReportReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		DB:     db,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Report")
		os.Exit(1)
	}
	if err = (&controllers.FetchDataReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FetchData")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	go func() {
		if err := runServer(context.Background(), db, setupLog, serverPort); err != nil {
			setupLog.Error(err, "failed to run http server")
			os.Exit(1)
		}
	}()
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func runServer(ctx context.Context, conn *pgx.Conn, logger logr.Logger, httpServerPort string) error {
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
