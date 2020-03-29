// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

func generateUtf8Password() (string, error) {
	buf := make([]byte, 100)
	_, err := rand.Read(buf)
	if err != nil {
		return "", fmt.Errorf("Failed to generate password: %w", err)
	}

	// Now that we have some random bytes, we need to turn it into valid
	// utf8 characters
	//
	// Example output:
	//
	//   7GOZOdMuQdjzJceJyla/72FkX0ymJDNNEyKKWVUxTP4IXtAUzYp8U0z0d8Wqh+p7J+K+D0NepgoEjqA79bBC6UkVtcorTFH+BBYaAetd3FsZdZ6V5Nn+UQ/VhpGNxU0fb7FOVg
	//
	password := base64.RawStdEncoding.EncodeToString(buf)

	return password, nil
}

func buildDefaultSuperuserSecret(dc *api.CassandraDatacenter) (*corev1.Secret, error) {
	var secret *corev1.Secret = nil

	if dc.ShouldGenerateSuperuserSecret() {
		secretNamespacedName := dc.GetSuperuserSecretNamespacedName()
		secret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretNamespacedName.Name,
				Namespace: secretNamespacedName.Namespace,
			},
		}
		username := dc.Spec.ClusterName + "-superuser"
		password, err := generateUtf8Password()
		if err != nil {
			return nil, fmt.Errorf("Failed to generate superuser password: %w", err)
		}

		secret.Data = map[string][]byte{
			"username": []byte(username),
			"password": []byte(password),
		}
	}

	return secret, nil
}

func (rc *ReconciliationContext) retrieveSuperuserSecret() (*corev1.Secret, error) {
	dc := rc.Datacenter
	secretNamespacedName := dc.GetSuperuserSecretNamespacedName()

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretNamespacedName.Name,
			Namespace: secretNamespacedName.Namespace,
		},
	}

	err := rc.Client.Get(
		rc.Ctx,
		secretNamespacedName,
		secret)

	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (rc *ReconciliationContext) retrieveSuperuserSecretOrCreateDefault() (*corev1.Secret, error) {
	dc := rc.Datacenter

	secret, retrieveErr := rc.retrieveSuperuserSecret()
	if retrieveErr != nil {
		if errors.IsNotFound(retrieveErr) {
			secret, err := buildDefaultSuperuserSecret(dc)

			if err == nil && secret == nil {
				return nil, retrieveErr
			}

			if err == nil {
				err = rc.Client.Create(rc.Ctx, secret)
			}

			if err != nil {
				return nil, fmt.Errorf("Failed to create default superuser secret: %w", err)
			}
		} else {
			return nil, retrieveErr
		}
	}

	return secret, nil
}


// Helper function that is easier to test
func validateSuperuserSecretContent(dc *api.CassandraDatacenter, secret *corev1.Secret) []error {
	var errs []error
	if secret != nil {
		namespacedName := types.NamespacedName{
			Name: secret.ObjectMeta.Name, 
			Namespace: secret.ObjectMeta.Namespace,
		}
		errorPrefix := fmt.Sprintf("Validation failed for superuser secret: %s", namespacedName.String())

		for _, key := range []string{"username", "password"} {
			value, ok := secret.Data[key]
			if !ok {
				errs = append(errs, fmt.Errorf("%s Missing key: %s", errorPrefix, key))
			} else if !utf8.Valid(value) {
				errs = append(errs, fmt.Errorf("%s Key did not have valid utf8 value: %s", errorPrefix, key))
			}
		}
	} else {
		if !dc.ShouldGenerateSuperuserSecret() {
			// We are not going to generate a secret and the user didn't provide us with
			// one, this is an error
			return []error{
				fmt.Errorf("Could not load superuser secret for CassandraCluster: %s", 
					dc.GetSuperuserSecretNamespacedName().String()),
			}
		}
	}
	return errs
}

func (rc *ReconciliationContext) validateSuperuserSecret() []error {
	secret, err := rc.retrieveSuperuserSecret()
	if err != nil {
		if errors.IsNotFound(err) {
			secret = nil
		} else {
			return []error{fmt.Errorf("Validation of superuser secret failed due to an error: %w", err)}
		}
	}
	return validateSuperuserSecretContent(rc.Datacenter, secret)
}
