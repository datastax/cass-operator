package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"github.com/pavel-v-chernykh/keystore-go"
	corev1 "k8s.io/api/core/v1"
	"math/big"
	"time"
)


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

func GetNewCAandKey(leafdomain, namespace string) (keypem, certpem string, err error) {
	serialNumber, notBefore, priv, privPem, notAfter, err := setupKey()
	if err == nil {
		buffer := bytes.NewBufferString("")
		template := x509.Certificate{
			SerialNumber: serialNumber,
			Subject: pkix.Name{
				CommonName:   fmt.Sprintf("%s.%s.svc", leafdomain, namespace),
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

func prepare_ca(ca *corev1.Secret) (ca_cert_bytes []byte, ca_certificate *x509.Certificate, ca_key *rsa.PrivateKey, err error) {
	ca_certificate_pem, _ := pem.Decode(ca.Data["cert"])
	ca_cert_bytes = ca_certificate_pem.Bytes
	ca_key_block, _ := pem.Decode(ca.Data["key"])
	if untyped_ca_key, ca_key_err := x509.ParsePKCS8PrivateKey(ca_key_block.Bytes); ca_key_err != nil {
		err = ca_key_err
		return
	} else {
		ca_key, _ = untyped_ca_key.(*rsa.PrivateKey)
	}
	ca_certificate, err = x509.ParseCertificate(ca_cert_bytes)
	return
}

func GenerateJKS(ca *corev1.Secret, podname, dcname string) (jksblob []byte, err error) {
	serialNumber, notBefore, priv, _, notAfter, err := setupKey()
	if err != nil {
		return nil, err
	}
	newCert := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("%s.%s.cassdc", podname, ca.ObjectMeta.Namespace),
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
	ca_cert_bytes, ca_certificate, ca_key, err := prepare_ca(ca)
	if derBytes, err = x509.CreateCertificate(rand.Reader, &newCert, ca_certificate, &priv.PublicKey, ca_key); err == nil {
		asn1_bytes, err := rsa2pkcs8(priv)
		buffer := bytes.NewBufferString("")
		fmt.Printf("Creating keystore")
		store := keystore.KeyStore{
			fmt.Sprintf("%s.%s.cassdc", podname, ca.ObjectMeta.Namespace): &keystore.PrivateKeyEntry{
				Entry:   keystore.Entry{CreationDate: time.Now()},
				PrivKey: asn1_bytes,
				CertChain: []keystore.Certificate{keystore.Certificate{
					Type:    "X509",
					Content: derBytes,
				}, keystore.Certificate{
					Type:    "X509",
					Content: ca_cert_bytes,
				}},
			},
			"ca": &keystore.TrustedCertificateEntry{
				Entry: keystore.Entry{CreationDate: time.Now()},
				Certificate: keystore.Certificate{
					Type:    "X509",
					Content: ca_cert_bytes,
				},
			}}
		err = keystore.Encode(buffer, store, []byte(dcname))
		return buffer.Bytes(), err
	}
	return nil, err

}

type pkcs8Key struct {
	Version             int
	PrivateKeyAlgorithm []asn1.ObjectIdentifier
	PrivateKey          []byte
}

func rsa2pkcs8(key *rsa.PrivateKey) ([]byte, error) {
	var pkey pkcs8Key
	pkey.Version = 0
	pkey.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 1)
	pkey.PrivateKeyAlgorithm[0] = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1} //RSA
	pkey.PrivateKey = x509.MarshalPKCS1PrivateKey(key)

	return asn1.Marshal(pkey)
}
