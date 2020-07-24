package utils

import (
	"crypto/rsa"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"io/ioutil"
)

func Test_newCA(t *testing.T) {
	var verify_key *rsa.PrivateKey
	pem_key, cert, err := GetNewCAandKey("cassandradatacenter-webhook-service","somenamespace")
	certpool := x509.NewCertPool()
	key, _ := pem.Decode([]byte(pem_key))
	if block, _ := pem.Decode([]byte(cert)); block != nil {
		var cert *x509.Certificate
		if cert, err = x509.ParseCertificate(block.Bytes); err == nil {
			certpool.AddCert(cert)
			verify_opts := x509.VerifyOptions{
				DNSName: "cassandradatacenter-webhook-service.somenamespace.svc",
				Roots:   certpool,
			}
			if _, err = cert.Verify(verify_opts); err != nil {
				t.Errorf("Error: %e", err)

			}
			var untyped_verify_key interface{}
			untyped_verify_key, err = x509.ParsePKCS8PrivateKey(key.Bytes)
			if err != nil {
				t.Errorf("Parsing key failed: %e", err)

			}
			var ok bool
			if verify_key, ok = untyped_verify_key.(*rsa.PrivateKey); !ok {
				t.Errorf("Error: couldn't typecast key")

			}
			if verify_cert_key, ok := cert.PublicKey.(*rsa.PublicKey); !ok {
				t.Errorf("Error: couldn't typecast cert key")


			} else {
				verify_key_public, _ := verify_key.Public().(*rsa.PublicKey)
				if fmt.Sprintf("%+v", verify_key_public) != fmt.Sprintf("%+v", verify_cert_key) {
					t.Errorf("Error: cert key public part mismatch: %+v %+v", verify_key_public, *verify_cert_key)
				}
			}

		}
	}
}

func Test_GetJKS(t *testing.T) {
	pem_key, cert, err := GetNewCAandKey("someclusterca", "somenamespace")
	if err != nil {
		t.Errorf("Got an error:: %e", err)
	}
	jks, err := GenerateJKS(&corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-keystore", "somedcname"),
			Namespace: "somedcnamespace",
		},
		Data: map[string][]byte{
			"cert": []byte(cert),
			"key":  []byte(pem_key),
		},
	}, "somepodname", "somedcname")
	if err != nil {
		t.Errorf("Got an error: %e", err)
	}
	if len(jks) == 0 {
		t.Errorf("JKS blob too small")
	}
	ioutil.WriteFile("test-jks", jks, 0644)

}
