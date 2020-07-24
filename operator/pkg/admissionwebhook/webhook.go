package webhook

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/datastax/cass-operator/operator/pkg/utils"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	altCertDir = filepath.Join(os.TempDir()) //Alt directory is necessary because regular key/cert mountpoint is read-only
	certDir    = filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs")

	serverCertFile    = filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs", "tls.crt")
	altServerCertFile = filepath.Join(altCertDir, "tls.crt")
	altServerKeyFile  = filepath.Join(altCertDir, "tls.key")

	log = logf.Log.WithName("cmd")
)

func EnsureWebhookCertificate(cfg *rest.Config) (certDir string, err error) {
	var contents []byte
	var webhook map[string]interface{}
	var bundled string
	var client crclient.Client
	namespace, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return "", err
	}
	var certpool *x509.CertPool
	if contents, err = ioutil.ReadFile(serverCertFile); err == nil && len(contents) > 0 {
		if client, err = crclient.New(cfg, crclient.Options{}); err == nil {
			if err, _, webhook, _ = fetchWebhookForNamespace(client, namespace); err == nil {
				if bundled, _, err = unstructured.NestedString(webhook, "clientConfig", "caBundle"); err == nil {
					if base64.StdEncoding.EncodeToString([]byte(contents)) == bundled {
						certpool, err = x509.SystemCertPool()
						if err != nil {
							certpool = x509.NewCertPool()
						}
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
									return certDir, nil
								}
							}
						}
					}
				}
			}
		}
	}
	return updateSecretAndWebhook(cfg, namespace)
}

func updateSecretAndWebhook(cfg *rest.Config, namespace string) (certDir string, err error) {
	var key, cert string
	var client crclient.Client
	if key, cert, err = utils.GetNewCAandKey("cass-operator-webhook-config", namespace); err == nil {
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
							return certDir, updateWebhook(client, cert, namespace)
						}
					}
				}

			}
		}
	}
	log.Error(err, "Failed to update certificates")
	return certDir, err
}

func fetchWebhookForNamespace(client crclient.Client, namespace string) (err error, webhook_config *unstructured.Unstructured, webhook map[string]interface{}, unstructured_index int) {

	webhook_config = &unstructured.Unstructured{}
	webhook_config.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "admissionregistration.k8s.io",
		Kind:    "ValidatingWebhookConfiguration",
		Version: "v1beta1",
	})
	err = client.Get(context.Background(), crclient.ObjectKey{
		Name: "cassandradatacenter-webhook-registration",
	}, webhook_config)
	if err != nil {
		return err, webhook_config, webhook, 0
	}
	var ok, present bool
	var found_namespace string
	var webhook_list []interface{}
	if webhook_list, present, err = unstructured.NestedSlice(webhook_config.Object, "webhooks"); err == nil {
		if present {
			for webhook_index, webhook_untypped := range webhook_list {
				webhook, ok = webhook_untypped.(map[string]interface{})
				if ok {
					if found_namespace, _, err = unstructured.NestedString(webhook, "clientConfig", "service", "namespace"); found_namespace == namespace {
						return nil, webhook_config, webhook, webhook_index
					}
				}
			}
		}
		return errors.New("Webhook not found for namespace"), webhook_config, webhook, 0
	}
	return err, webhook_config, webhook, 0
}

func updateWebhook(client crclient.Client, cert, namespace string) (err error) {
	var webhook_slice []interface{}
	var webhook map[string]interface{}
	var present bool
	var webhook_index int
	var webhook_config *unstructured.Unstructured
	err, webhook_config, webhook, webhook_index = fetchWebhookForNamespace(client, namespace)
	if err == nil {
		if err = unstructured.SetNestedField(webhook, namespace, "clientConfig", "service", "namespace"); err == nil {
			if err = unstructured.SetNestedField(webhook, base64.StdEncoding.EncodeToString([]byte(cert)), "clientConfig", "caBundle"); err == nil {
				if webhook_slice, present, err = unstructured.NestedSlice(webhook_config.Object, "webhooks"); present && err == nil {
					webhook_slice[webhook_index] = webhook
					if err = unstructured.SetNestedSlice(webhook_config.Object, webhook_slice, "webhooks"); err == nil {
						err = client.Update(context.Background(), webhook_config)
					}
				}
			}
		}
	}
	return err
}

func EnsureWebhookConfigVolume(cfg *rest.Config) (err error) {
	var pod *v1.Pod
	namespace, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		return err
	}
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
