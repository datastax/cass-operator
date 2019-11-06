package httphelper

import (
	"context"
	"fmt"
	"net/http"
	"crypto/x509"
	"crypto/tls"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
)

// API for Node Management mAuth Config
func GetManagementApiProtocol(dseDatacenter *datastaxv1alpha1.DseDatacenter) (string, error) {
	provider, err := BuildManagmenetApiSecurityProvider(dseDatacenter)
	if err != nil {
		return "", err
	}
	return provider.GetProtocol(), nil
}

func BuildManagementApiHttpClient(dseDatacenter *datastaxv1alpha1.DseDatacenter, client client.Client, ctx context.Context) (HttpClient, error) {
	provider, err := BuildManagmenetApiSecurityProvider(dseDatacenter)
	if err != nil {
		return nil, err
	}
	return provider.BuildHttpClient(client, ctx)
}

func AddManagementApiServerSecurity(dseDatacenter *datastaxv1alpha1.DseDatacenter, dsePod *corev1.PodTemplateSpec) error {
	provider, err := BuildManagmenetApiSecurityProvider(dseDatacenter)
	if err != nil {
		return err
	}
	provider.AddServerSecurity(dsePod)

	return nil
}

func BuildManagmenetApiSecurityProvider(dseDatacenter *datastaxv1alpha1.DseDatacenter) (ManagementApiSecurityProvider, error) {
	options := []func(*datastaxv1alpha1.DseDatacenter)(ManagementApiSecurityProvider,error){
		buildManualApiSecurityProvider,
		buildInsecureManagementApiSecurityProvider}

	var selectedProvider ManagementApiSecurityProvider = nil

	for _, builder := range options {
		provider, err := builder(dseDatacenter)
		if err != nil {
			return nil, err
		}
		if provider != nil && selectedProvider != nil {
			return nil, fmt.Errorf("Multiple options specified for 'managementApiAuth', but expected exactly one.")
		}
		if provider != nil {
			selectedProvider = provider
		}
	}

	if selectedProvider == nil {
		return nil, fmt.Errorf("No security strategy specified for 'managementApiAuth'.")
	}

	return selectedProvider, nil
}


// SPI for adding new mechanisms for securing the management API
type ManagementApiSecurityProvider interface {
	BuildHttpClient(client client.Client, ctx context.Context) (HttpClient, error)
	AddServerSecurity(dsePod *corev1.PodTemplateSpec) error
	GetProtocol() string
}

type InsecureManagementApiSecurityProvider struct {

}

func buildInsecureManagementApiSecurityProvider(dseDatacenter *datastaxv1alpha1.DseDatacenter) (ManagementApiSecurityProvider, error) {
	if dseDatacenter.Spec.ManagementApiAuth.Insecure != nil {
		return &InsecureManagementApiSecurityProvider{}, nil
	}
	return nil, nil
}

func (provider *InsecureManagementApiSecurityProvider) GetProtocol() string {
	return "http"
}

func (provider *InsecureManagementApiSecurityProvider) BuildHttpClient(client client.Client, ctx context.Context) (HttpClient, error) {
	return http.DefaultClient, nil
}

func (provider *InsecureManagementApiSecurityProvider) AddServerSecurity(dsePod *corev1.PodTemplateSpec) error {
	return nil
}

type ManualManagementApiSecurityProvider struct {
	Namespace string
	Config *datastaxv1alpha1.ManagementApiAuthManualConfig
}

func buildManualApiSecurityProvider(dseDatacenter *datastaxv1alpha1.DseDatacenter) (ManagementApiSecurityProvider, error) {
	if dseDatacenter.Spec.ManagementApiAuth.Manual != nil {
		provider := &ManualManagementApiSecurityProvider{}
		provider.Config = dseDatacenter.Spec.ManagementApiAuth.Manual
		provider.Namespace = dseDatacenter.ObjectMeta.Namespace
		return provider, nil
	}
	return nil, nil
}

func (provider *ManualManagementApiSecurityProvider) GetProtocol() string {
	return "https"
}

func (provider *ManualManagementApiSecurityProvider) AddServerSecurity(dsePod *corev1.PodTemplateSpec) error {
	caCertPath := "/management-api-certs/ca.crt"
	tlsCrt := "/management-api-certs/tls.crt"
	tlsKey := "/management-api-certs/tls.key"


	// Find the DSE container
	var dseContainer *corev1.Container = nil
	for i, _ := range dsePod.Spec.Containers {
		if dsePod.Spec.Containers[i].Name == "dse" {
			dseContainer = &dsePod.Spec.Containers[i]
		}
	}

	if dseContainer == nil {
		return fmt.Errorf("Could not find container with name '%s'", "dse")
	}


	// Add volume containing certificates
	secretVolumeName := "management-api-server-certs-volume"
	secretVolume := corev1.Volume{
		Name: secretVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: provider.Config.ServerSecretName,
			},
		},
	}

	if dsePod.Spec.Volumes == nil {
		dsePod.Spec.Volumes = []corev1.Volume{}
	}

	dsePod.Spec.Volumes = append(dsePod.Spec.Volumes, secretVolume)


	// Mount certificates volume in DSE container
	secretVolumeMount := corev1.VolumeMount{
		Name: secretVolumeName,
		ReadOnly: true,
		MountPath: "/management-api-certs",
	}

	if dseContainer.VolumeMounts == nil {
		dseContainer.VolumeMounts = []corev1.VolumeMount{}
	}

	dseContainer.VolumeMounts = append(dseContainer.VolumeMounts, secretVolumeMount)


	// Configure DSE Management API to use certificates
	envVars := []corev1.EnvVar{
		{
			Name: "DSE_MGMT_TLS_CA_CERT_FILE",
			Value: caCertPath,
		},
		{
			Name: "DSE_MGMT_TLS_CERT_FILE",
			Value: tlsCrt,
		},
		{
			Name: "DSE_MGMT_TLS_KEY_FILE",
			Value: tlsKey,
		},
	}

	if dseContainer.Env == nil {
		dseContainer.Env = []corev1.EnvVar{}
	}

	dseContainer.Env = append(envVars, dseContainer.Env...)


	// Update Liveness probe to account for mutual auth (can't just use HTTP probe now)
	// TODO: Get endpoint from configured HTTPGet probe
	livenessEndpoint := "https://localhost:8080/api/v0/probes/liveness"
	if dseContainer.LivenessProbe == nil {
		dseContainer.LivenessProbe =  &corev1.Probe{
			Handler: corev1.Handler{
			},
		}
	}
	dseContainer.LivenessProbe.Handler.HTTPGet = nil
	dseContainer.LivenessProbe.Handler.TCPSocket = nil
	dseContainer.LivenessProbe.Handler.Exec = &corev1.ExecAction{
		Command: []string{"wget",
			"--output-document", "/dev/null",
			"--no-check-certificate",
			"--certificate", tlsCrt,
			"--private-key", tlsKey,
			"--ca-certificate", caCertPath,
			livenessEndpoint},
	}


	// Update Readiness probe to account for mutual auth (can't just use HTTP probe now)
	// TODO: Get endpoint from configured HTTPGet probe
	readinessEndpoint := "https://localhost:8080/api/v0/probes/readiness"
	if dseContainer.ReadinessProbe == nil {
		dseContainer.ReadinessProbe =  &corev1.Probe{
			Handler: corev1.Handler{
			},
		}
	}
	dseContainer.ReadinessProbe.Handler.HTTPGet = nil
	dseContainer.ReadinessProbe.Handler.TCPSocket = nil
	dseContainer.ReadinessProbe.Handler.Exec = &corev1.ExecAction{
		Command: []string{"wget",
			"--output-document", "/dev/null",
			"--no-check-certificate",
			"--certificate", tlsCrt,
			"--private-key", tlsKey,
			"--ca-certificate", caCertPath,
			readinessEndpoint},
	}

	return nil
}

func validateSecret(secret *corev1.Secret) error {
	secretNamespacedName := types.NamespacedName{
		Name:      secret.ObjectMeta.Name,
		Namespace: secret.ObjectMeta.Namespace,}

	if secret.Type != "kubernetes.io/tls" {
		// Not the right type
		err := fmt.Errorf("Expected Secret %s to have type 'kubernetes.io/tls' but was '%s'",
			secretNamespacedName.String(),
			secret.Type)
		return err
	}

	for _, key := range []string{"ca.crt", "tls.crt", "tls.key"} {
		if _, ok := secret.Data[key]; !ok {
			err := fmt.Errorf("Expected Secret %s to have data key '%s' but was not found",
				secretNamespacedName.String(),
				key)
			return err
		}
	}

	return nil
}

func (provider *ManualManagementApiSecurityProvider) BuildHttpClient(client client.Client, ctx context.Context) (HttpClient, error) {
	// Get the client Secret
	secretNamespacedName := types.NamespacedName{
		Name:      provider.Config.ClientSecretName,
		Namespace: provider.Namespace,}

	secret := &corev1.Secret{}
	err := client.Get(
		ctx,
		secretNamespacedName,
		secret)

	if err != nil {
		// Couldn't get the secret
		return nil, err
	}

	err = validateSecret(secret)
	if err != nil {
		// Secret didn't look the way we expect
		return nil, err
	}

	// Create the CA certificate pool
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(secret.Data["ca.crt"])
	if !ok {
		err = fmt.Errorf("No certificates found in %s when parsing 'ca.crt' value: %v",
			secretNamespacedName.String(),
			secret.Data["ca.crt"])
		return nil, err
	}

	// Load client key pair
	cert, err := tls.X509KeyPair(
		secret.Data["tls.crt"],
		secret.Data["tls.key"])

	// Build the client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs: caCertPool,
        // TODO: ...we should probably verify something here...
		InsecureSkipVerify: true,
		VerifyPeerCertificate: buildVerifyPeerCertificateNoHostCheck(caCertPool),
	}
	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	httpClient := &http.Client{Transport: transport}

	return httpClient, nil
}

// Below implementation modified from:
//
// https://go-review.googlesource.com/c/go/+/193620/5/src/crypto/tls/example_test.go#210
//
func buildVerifyPeerCertificateNoHostCheck(RootCAs *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	f := func(certificates [][]byte, _ [][]*x509.Certificate) error {
		certs := make([]*x509.Certificate, len(certificates))
		for i, asn1Data := range certificates {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				return err
			}
			certs[i] = cert
		}

		opts := x509.VerifyOptions{
			Roots:         RootCAs,
			// Setting the DNSName to the empty string will cause
			// Certificate.Verify() to skip hostname checking
			DNSName:       "",
			Intermediates: x509.NewCertPool(),
		}
		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}
		_, err := certs[0].Verify(opts)
		return err
	}
	return f
}
