// Copyright DataStax, Inc.
// Please see the included license file for details.

package main

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"github.com/datastax/cass-operator/operator/pkg/apis"
	"github.com/datastax/cass-operator/operator/pkg/controller"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	kubemetrics "github.com/operator-framework/operator-sdk/pkg/kube-metrics"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	"github.com/operator-framework/operator-sdk/pkg/ready"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	controllerRuntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost               = "0.0.0.0"
	metricsPort         int32 = 8383
	operatorMetricsPort int32 = 8686
	version                   = "DEV"

	altCertDir = filepath.Join(os.TempDir()) //Alt directory is necessary because regular key/cert mountpoint is read-only
	certDir    = filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs")

	serverCertFile    = filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs", "tls.crt")
	altServerCertFile = filepath.Join(altCertDir, "tls.crt")
	altServerKeyFile  = filepath.Join(altCertDir, "tls.key")
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

	// Use the operator-sdk ready pkg
	readyFile := ready.NewFileReady()
	err = readyFile.Set()
	if err != nil {
		log.Error(err, "Problem creating readyFile. Exited non-zero")
		os.Exit(1)
	}
	log.Info("created the readyFile.")
	defer readyFile.Unset()

	ctx := context.Background()
	// Become the leader before proceeding
	err = leader.Become(ctx, "cass-operator-lock")
	if err != nil {
		log.Error(err, "could not become leader")
		os.Exit(1)
	}

	if err = ensureWebhookConfigVolume(cfg, namespace); err != nil {
		log.Error(err, "Failed to ensure webhook volume")
	}
	if err = ensureWebhookCertificate(cfg, namespace); err != nil {
		log.Error(err, "Failed to ensure webhook CA configuration")
	}

	// Set default manager options
	options := manager.Options{
		Namespace:          namespace,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		CertDir:            certDir,
		Port:               8443,
	}

	// Add support for MultiNamespace set in WATCH_NAMESPACE (e.g ns1,ns2)
	// Note that this is not intended to be used for excluding namespaces, this is better done via a Predicate
	// Also note that you may face performance issues when using this with a high number of namespaces.
	// More Info: https://godoc.org/github.com/kubernetes-sigs/controller-runtime/pkg/cache#MultiNamespacedCacheBuilder
	if strings.Contains(namespace, ",") {
		options.Namespace = ""
		options.NewCache = cache.MultiNamespacedCacheBuilder(strings.Split(namespace, ","))
	}

	// Create a new manager to provide shared dependencies and start components
	mgr, err := manager.New(cfg, options)
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

	// Add the Metrics Service
	addMetrics(ctx, cfg)

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

// addMetrics will create the Services and Service Monitors to allow the operator export the metrics by using
// the Prometheus operator
func addMetrics(ctx context.Context, cfg *rest.Config) {
	// Get the namespace the operator is currently deployed in.
	operatorNs, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		if errors.Is(err, k8sutil.ErrRunLocal) {
			log.Info("Skipping CR metrics server creation; not running in a cluster.")
			return
		}
	}

	if err := serveCRMetrics(cfg, operatorNs); err != nil {
		log.Info("Could not generate and serve custom resource metrics", "error", err.Error())
	}

	// Add to the below struct any other metrics ports you want to expose.
	servicePorts := []v1.ServicePort{
		{Port: metricsPort, Name: metrics.OperatorPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: metricsPort}},
		{Port: operatorMetricsPort, Name: metrics.CRPortName, Protocol: v1.ProtocolTCP, TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: operatorMetricsPort}},
	}

	// Create Service object to expose the metrics port(s).
	service, err := metrics.CreateMetricsService(ctx, cfg, servicePorts)
	if err != nil {
		log.Info("Could not create metrics Service", "error", err.Error())
	}

	// CreateServiceMonitors will automatically create the prometheus-operator ServiceMonitor resources
	// necessary to configure Prometheus to scrape metrics from this operator.
	services := []*v1.Service{service}

	// The ServiceMonitor is created in the same namespace where the operator is deployed
	_, err = metrics.CreateServiceMonitors(cfg, operatorNs, services)
	if err != nil {
		log.Info("Could not create ServiceMonitor object", "error", err.Error())
		// If this operator is deployed to a cluster without the prometheus-operator running, it will return
		// ErrServiceMonitorNotPresent, which can be used to safely skip ServiceMonitor creation.
		if err == metrics.ErrServiceMonitorNotPresent {
			log.Info("Install prometheus-operator in your cluster to create ServiceMonitor objects", "error", err.Error())
		}
	}
}

// serveCRMetrics gets the Operator/CustomResource GVKs and generates metrics based on those types.
// It serves those metrics on "http://metricsHost:operatorMetricsPort".
func serveCRMetrics(cfg *rest.Config, operatorNs string) error {
	// The function below returns a list of filtered operator/CR specific GVKs. For more control, override the GVK list below
	// with your own custom logic. Note that if you are adding third party API schemas, probably you will need to
	// customize this implementation to avoid permissions issues.
	filteredGVK, err := k8sutil.GetGVKsFromAddToScheme(apis.AddToScheme)
	if err != nil {
		return err
	}

	// The metrics will be generated from the namespaces which are returned here.
	// NOTE that passing nil or an empty list of namespaces in GenerateAndServeCRMetrics will result in an error.
	ns, err := kubemetrics.GetNamespacesForMetrics(operatorNs)
	if err != nil {
		return err
	}

	// Generate and serve custom resource specific metrics.
	err = kubemetrics.GenerateAndServeCRMetrics(cfg, ns, filteredGVK, metricsHost, operatorMetricsPort)
	if err != nil {
		return err
	}
	return nil
}

func ensureWebhookCertificate(cfg *rest.Config, namespace string) (err error) {
	var contents []byte
	if contents, err = ioutil.ReadFile(serverCertFile); err == nil && len(contents) > 0 {
		certpool := x509.NewCertPool()
		var block *pem.Block
		if block, _ = pem.Decode(contents); err == nil && block != nil {
			var cert *x509.Certificate
			if cert, err = x509.ParseCertificate(block.Bytes); err == nil {
				certpool.AddCert(cert)
				log.Info("Attempting to validate operator CA")
				verify_opts := x509.VerifyOptions{
					DNSName: fmt.Sprintf("cassandradatacenter-webhook-service.%s.svc", namespace),
					Roots:   certpool,
				}
				if _, err = cert.Verify(verify_opts); err == nil {
					log.Info("Found valid certificate for webhook")
					return nil
				}
			}
		}
	}
	return updateSecretAndWebhook(cfg, namespace)
}

func updateSecretAndWebhook(cfg *rest.Config, namespace string) (err error) {
	var key, cert string
	var client crclient.Client
	if key, cert, err = getNewCertAndKey(namespace); err == nil {
		if client, err = crclient.New(cfg, crclient.Options{}); err == nil {
			secret := &v1.Secret{}
			err = client.Get(context.Background(), crclient.ObjectKey{
				Namespace: namespace,
				Name:      "cass-operator-webhook-config",
			}, secret)
			if err == nil {
				secret.StringData = make(map[string]string)
				secret.StringData["tls.key"] = key
				secret.StringData["tls.crt"] = cert
				if err = client.Update(context.Background(), secret); err == nil {
					log.Info("TLS secret for webhook updated")
					if err = ioutil.WriteFile(altServerCertFile, []byte(cert), 0600); err == nil {
						if err = ioutil.WriteFile(altServerKeyFile, []byte(key), 0600); err == nil {
							certDir = altCertDir
							log.Info("TLS secret updated in pod mount")
							return updateWebhook(client, cert, namespace)
						}
					}
				}

			}
		}
	}
	log.Error(err, "Failed to update certificates")
	return err
}

func updateWebhook(client crclient.Client, cert, namespace string) (err error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "admissionregistration.k8s.io",
		Kind:    "ValidatingWebhookConfiguration",
		Version: "v1beta1",
	})
	err = client.Get(context.Background(), crclient.ObjectKey{
		Name: "cassandradatacenter-webhook-registration",
	}, u)
	if err == nil {
		var webhook_slice []interface{}
		var webhook map[string]interface{}
		var ok, present bool
		webhook_slice, present, err = unstructured.NestedSlice(u.Object, "webhooks")
		webhook, ok = webhook_slice[0].(map[string]interface{})
		if !ok || !present || err != nil {
			log.Info(fmt.Sprintf("Error loading webhook for modification: %+v %+v %+v", ok, present, err))
			return err
		}
		if err = unstructured.SetNestedField(webhook, namespace, "clientConfig", "service", "namespace"); err == nil {
			if err = unstructured.SetNestedField(webhook, base64.StdEncoding.EncodeToString([]byte(cert)), "clientConfig", "caBundle"); err == nil {
				webhook_slice[0] = webhook
				if err = unstructured.SetNestedSlice(u.Object, webhook_slice, "webhooks"); err == nil {
					err = client.Update(context.Background(), u)
				}
			}
		}
	}
	return err
}

func ensureWebhookConfigVolume(cfg *rest.Config, namespace string) (err error) {
	var pod *v1.Pod
	var client crclient.Client
	if client, err = crclient.New(cfg, crclient.Options{}); err == nil {
		if pod, err = k8sutil.GetPod(context.Background(), client, namespace); err == nil {
			for _, volume := range pod.Spec.Volumes {
				if "cass-operator-certs-volume" == volume.Name {
					return nil
				}
			}
			log.Error(fmt.Errorf("Secrets volume not found, unable to start webhook"), "")
			os.Exit(1)
		}
	}
	return err
}
