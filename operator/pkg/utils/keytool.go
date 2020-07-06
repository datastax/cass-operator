package utils

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"os"
	"time"

	"github.com/pavel-v-chernykh/keystore-go"
	"github.com/sethvargo/go-password/password"
)

const (
	keyStoreType      string = "KEYSTORE"
	keyStoreName      string = "identity.jks"
	keyStoreAlias     string = "client"
	trustStoreType    string = "TRUSTSTORE"
	trustStoreName    string = "trustStore.jks"
	trustStoreAlias   string = "remote"
	certificateFormat string = "X509"
	passwordLength    int    = 17
	passwordDigits    int    = 5
)


type Location struct {
	BasePath string `json:"basePath"`
	FileName string `json:"fileName"`
}

// Path returns the fully qualified path of the file this location represents
func (l *Location) Path() string {
	return filepath.Join(l.BasePath, l.FileName)
}

// String returns the string representation of Location
func (l *Location) String() string {
	return fmt.Sprintf("Location{Path: %s}", l.Path())
}

// Keytool ...
type Keytool struct {
}

// NewKeytool ...
func NewKeytool() *Keytool {
	return &Keytool{}
}

func path() string {
	pwd, _ := os.Getwd()
	return pwd
}

type KeyStore struct {
	Type     string
	Alias    string
	Location *Location
	Password *string
}

type CredentialsMetadata struct {
	CertificateAuthorityLocation           *Location `json:"certificateAuthorityLocation"`
	CertificateLocation                    *Location `json:"certificateLocation"`
	CertificatePrivateKeyLocation          *Location `json:"certificatePrivateKeyLocation"`
}

// String returns the string representation of CredentialsMetadata
func (c CredentialsMetadata) String() string {
	return fmt.Sprintf("CredentialsMetadata{certificateAuthorityLocation: %s,  certificateLocation: %s, certificatePrivateKeyLocation: %s}",
		c.CertificateAuthorityLocation, c.CertificateLocation, c.CertificatePrivateKeyLocation)
}

// CreateKeyStore ...
func (k *Keytool) CreateKeyStore(key, cert []byte) (*KeyStore, error) {
	var keyStore KeyStore
	var err error

	keyStore.Type = keyStoreType
	keyStore.Alias = keyStoreAlias
	keyStore.Location = &Location{BasePath: path(), FileName: keyStoreName}
	password := generatePassword()
	keyStore.Password = &password

	if err != nil {
		log.Info("problem reading key")
		return nil, err
	}

	if err != nil {
		log.Info("problem reading certificate")
		return nil, err
	}

	store, err := createIdentityStore(keyStore.Alias, key, cert)
	if err != nil {
		log.Info("problem creating keystore")
		return nil, err
	}

	err = writeKeyStore(store, keyStore.Location.Path(), *keyStore.Password)
	if err != nil {
		log.Info("problem writing keystore")
		return nil, err
	}

	return &keyStore, nil
}

// CreateTrustStore ...
func (k *Keytool) CreateTrustStore(caCert []byte) (*KeyStore, error) {
	var trustStore KeyStore
	var err error

	trustStore.Type = trustStoreType
	trustStore.Alias = trustStoreAlias
	trustStore.Location = &Location{BasePath: path(), FileName: trustStoreName}
	password := generatePassword()
	trustStore.Password = &password

	if err != nil {
		log.Info("problem reading issuing CA")
		return nil, err
	}

	store := createTrustStore(trustStore.Alias, caCert)

	err = writeKeyStore(store, trustStore.Location.Path(), *trustStore.Password)
	if err != nil {
		log.Info("problem writing truststore")
		return nil, err
	}

	return &trustStore, nil
}

func createIdentityStore(alias string, key []byte, cert []byte) (keystore.KeyStore, error) {
	pvtKey, _ := pem.Decode(key)
	privateKey, err := x509.ParsePKCS1PrivateKey(pvtKey.Bytes)
	if err != nil {
		return nil, err
	}
	pkcs8, err := rsa2pkcs8(privateKey)
	if err != nil {
		return nil, err
	}

	certificate := keystore.Certificate{
		Type:    certificateFormat,
		Content: cert,
	}
	certificateChain := []keystore.Certificate{certificate}

	identityStore := keystore.KeyStore{
		alias: &keystore.PrivateKeyEntry{
			Entry: keystore.Entry{
				CreationDate: time.Now(),
			},
			PrivKey:   pkcs8,
			CertChain: certificateChain,
		},
	}
	return identityStore, nil
}

func createTrustStore(alias string, remoteCert []byte) keystore.KeyStore {
	serverCertificate := keystore.Certificate{
		Type:    certificateFormat,
		Content: remoteCert,
	}

	trustStore := keystore.KeyStore{
		alias: &keystore.TrustedCertificateEntry{
			Entry: keystore.Entry{
				CreationDate: time.Now(),
			},
			Certificate: serverCertificate,
		},
	}
	return trustStore
}

func writeKeyStore(keyStore keystore.KeyStore, filename, password string) error {
	o, err := os.Create(filename)
	defer o.Close()
	if err != nil {
		return err
	}

	err = keystore.Encode(o, keyStore, []byte(password))
	if err != nil {
		return err
	}

	return nil
}

func generatePassword() string {
	psswrd, _ := password.Generate(passwordLength, passwordDigits, 0, false, false)
	return psswrd
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
