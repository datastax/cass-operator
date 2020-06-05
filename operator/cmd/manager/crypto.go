package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

func getNewCertAndKey(namespace string) (keypem, certpem string, err error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	var serialNumber *big.Int
	if serialNumber, err = rand.Int(rand.Reader, serialNumberLimit); err == nil {
		notBefore := time.Now()
		var priv *rsa.PrivateKey
		if priv, err = rsa.GenerateKey(rand.Reader, 4096); err == nil {
			buffer := bytes.NewBufferString("")
			notAfter := notBefore.Add(365 * 24 * time.Hour)
			template := x509.Certificate{
				SerialNumber: serialNumber,
				Subject: pkix.Name{
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
					var privBytes []byte
					if privBytes, err = x509.MarshalPKCS8PrivateKey(priv); err == nil {
						buffer.Reset()
						if err = pem.Encode(buffer, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err == nil {
							return buffer.String(), cert, nil
						}
					}
				}
			}
		}
	}
	return "", "", err
}
