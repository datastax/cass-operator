// Copyright DataStax, Inc.
// Please see the included license file for details.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	kubemetrics "github.com/operator-framework/operator-sdk/pkg/kube-metrics"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	"github.com/operator-framework/operator-sdk/pkg/restmapper"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	controllerRuntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/datastax/cass-operator/operator/pkg/apis"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/controller"
	"github.com/operator-framework/operator-sdk/pkg/ready"
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost               = "0.0.0.0"
	metricsPort         int32 = 8383
	operatorMetricsPort int32 = 8686
	version                   = "DEV"
)
var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info("Go Version",
		"goVersion", runtime.Version())

	log.Info("Go OS/Arch",
		"os", runtime.GOOS,
		"arch", runtime.GOARCH)

	log.Info("Version of operator-sdk",
		"operatorSdkVersion", sdkVersion.Version)

	log.Info("Operator version",
		"operatorVersion", version)
}

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.Parse()

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.Logger())

	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "could not get k8s config")
		os.Exit(1)
	}

	ctx := context.Background()

	// Use the operator-sdk ready pkg
	readyFile := ready.NewFileReady()
	err = readyFile.Set()
	if err != nil {
		log.Error(err, "Problem creating readyFile. Exited non-zero")
		os.Exit(1)
	}
	log.Info("created the readyFile.")
	defer readyFile.Unset()

	// Become the leader before proceeding
	err = leader.Become(ctx, "cass-operator-lock")
	if err != nil {
		log.Error(err, "could not become leader")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace:          namespace,
		MapperProvider:     restmapper.NewDynamicRESTMapper,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		log.Error(err, "could not make manager")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "could not add to scheme")
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "could not add to manager")
		os.Exit(1)
	}

	skipWebhookEnvVal := os.Getenv("SKIP_VALIDATING_WEBHOOK")
	if skipWebhookEnvVal == "" {
		skipWebhookEnvVal = "FALSE"
	}
	skipWebhook, err := strconv.ParseBool(skipWebhookEnvVal)
	if err != nil {
		log.Error(err, "bad value for SKIP_VALIDATING_WEBHOOK env")
		os.Exit(1)
	}

	if !skipWebhook {
		err = controllerRuntime.NewWebhookManagedBy(mgr).For(&api.CassandraDatacenter{}).Complete()
		if err != nil {
			log.Error(err, "unable to create validating webhook for CassandraDatacenter")
			os.Exit(1)
		}
	}

	if err = serveCRMetrics(cfg); err != nil {
		log.Error(err, "Could not generate and serve custom resource metrics", "error")
	}

	// Add to the below struct any other metrics ports you want to expose.
	servicePorts := []v1.ServicePort{
		{Port: metricsPort, Name: metrics.OperatorPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: metricsPort}},
		{Port: operatorMetricsPort, Name: metrics.CRPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: operatorMetricsPort}},
	}
	// Create Service object to expose the metrics port(s).
	_, err = metrics.CreateMetricsService(ctx, cfg, servicePorts)
	if err != nil {
		log.Error(err, "could not expose metrics port, continuing anyway")
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

// serveCRMetrics gets the Operator/CustomResource GVKs and generates metrics based on those types.
// It serves those metrics on "http://metricsHost:operatorMetricsPort".
func serveCRMetrics(cfg *rest.Config) error {
	// Below function returns filtered operator/CustomResource specific GVKs.
	// For more control override the below GVK list with your own custom logic.
	filteredGVK, err := k8sutil.GetGVKsFromAddToScheme(apis.AddToScheme)
	if err != nil {
		return err
	}
	// Get the namespace the operator is currently deployed in.
	operatorNs, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}
	// To generate metrics in other namespaces, add the values below.
	ns := []string{operatorNs}
	// Generate and serve custom resource specific metrics.
	err = kubemetrics.GenerateAndServeCRMetrics(cfg, ns, filteredGVK, metricsHost, operatorMetricsPort)
	if err != nil {
		return err
	}
	return nil
}
