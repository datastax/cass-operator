package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"github.com/pavel-v-chernykh/keystore-go"
	corev1 "k8s.io/api/core/v1"
	"math/big"
	"time"
)

var log = logf.Log.WithName("keytool")

func setupKey() (*big.Int, time.Time, *rsa.PrivateKey, string, time.Time, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	var serialNumber *big.Int
	buffer := bytes.NewBufferString("")
	var privBytes []byte
	var err error
	if serialNumber, err = rand.Int(rand.Reader, serialNumberLimit); err == nil {
		notBefore := time.Now()
		var priv *rsa.PrivateKey
		if priv, err = rsa.GenerateKey(rand.Reader, 4096); err == nil {
			notAfter := notBefore.Add(365 * 24 * time.Hour)
			if privBytes, err = x509.MarshalPKCS8PrivateKey(priv); err == nil {
				if err = pem.Encode(buffer, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err == nil {
					return serialNumber, notBefore, priv, buffer.String(), notAfter, err

				}
			}
		}
	}

	return nil, time.Time{}, nil, "", time.Time{}, err
}

func GetNewCAandKey(namespace string) (keypem, certpem string, err error) {
	serialNumber, notBefore, priv, privPem, notAfter, err := setupKey()
	if err == nil {
		buffer := bytes.NewBufferString("")
		template := x509.Certificate{
			SerialNumber: serialNumber,
			Subject: pkix.Name{
				CommonName: fmt.Sprintf("cassandradatacenter-webhook-service.%s.svc", namespace),
				Organization: []string{"Cassandra Kubernetes Operator By Datastax"},
			},
			NotBefore: notBefore,
			NotAfter:  notAfter,

			IsCA:                  true,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
			DNSNames:              []string{fmt.Sprintf("cassandradatacenter-webhook-service.%s.svc", namespace)},
		}
		var derBytes []byte
		if derBytes, err = x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv); err == nil {
			if err = pem.Encode(buffer, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err == nil {
				cert := buffer.String()
				return privPem, cert, nil
			}
		}
	}
	return "", "", err
}

func GenerateJKS(ca *corev1.Secret, podname, dcname string) (jksblob []byte, err error) {
	serialNumber, notBefore, priv, _, notAfter, err := setupKey()
	newCert := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s.%s.cassdc", podname, ca.ObjectMeta.Namespace),
			Organization: []string{"Cassandra Kubernetes Operator By Datastax"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		IsCA:                  false,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{fmt.Sprintf("%s.%s.cassdc", podname, ca.ObjectMeta.Namespace)},
	}
	var derBytes []byte
	fmt.Printf("Decoding Cert to embed in JKS")
	ca_certificate_pem, _ := pem.Decode(ca.Data["cert"])
	ca_key_block, _ := pem.Decode(ca.Data["key"])
	var ca_key *rsa.PrivateKey
	if untyped_ca_key, ca_key_err := x509.ParsePKCS8PrivateKey(ca_key_block.Bytes); ca_key_err != nil {
		return nil, ca_key_err
	} else {
		ca_key, _ = untyped_ca_key.(*rsa.PrivateKey)
	}
	var ca_certificate *x509.Certificate
	ca_certificate, err = x509.ParseCertificate(ca_certificate_pem.Bytes)
	buffer := bytes.NewBufferString("")
	asn1_bytes, err := rsa2pkcs8(ca_key)
	if derBytes, err = x509.CreateCertificate(rand.Reader, &newCert, ca_certificate, &priv.PublicKey, ca_key); err == nil {
		fmt.Printf("Creating keystore")
		store := keystore.KeyStore{
			fmt.Sprintf("%s.%s.cassdc", podname, ca.ObjectMeta.Namespace): &keystore.PrivateKeyEntry{
				Entry:   keystore.Entry{CreationDate: time.Now()},
				PrivKey: asn1_bytes,
				CertChain: []keystore.Certificate{keystore.Certificate{
					Type:    "X509",
					Content: derBytes,
				},keystore.Certificate{
					Type:    "X509",
					Content: ca_certificate_pem.Bytes,
				}},
			},
			"ca": &keystore.TrustedCertificateEntry{
				Entry: keystore.Entry{CreationDate: time.Now()},
				Certificate: keystore.Certificate{
					Type:    "X509",
					Content: ca_certificate_pem.Bytes,
				},
			}}
		err = keystore.Encode(buffer, store, []byte(dcname))
	}
	return buffer.Bytes(), err

}

func GetSignedCertAndKey(cakeypem, cacertpem string, sans ...string) (signedcertpem, keypem string, err error) {
	return "", "", nil
}
