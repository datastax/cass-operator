package httphelper

import (
	"testing"
	"crypto/x509"
	"encoding/pem"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
)

func Test_BuildPodHostFromPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-foo",
			Namespace: "somenamespace",
			Labels: map[string]string{
				datastaxv1alpha1.DatacenterLabel: "dc-bar",
				datastaxv1alpha1.ClusterLabel:    "the-foobar-cluster",
			},
		},
	}

	result := BuildPodHostFromPod(pod)
	expected := "pod-foo.the-foobar-cluster-dc-bar-service.somenamespace"

	assert.Equal(t, expected, result)
}

func Test_buildVerifyPeerCertificateNoHostCheck_AcceptsGoodCert(t *testing.T) {
	// openssl req -extensions v3_ca -new -x509 -days 36500 -nodes \
	//   -subj "/CN=MyRootCA" -newkey rsa:2048 -sha512 -out ca.crt \
	//   -keyout ca.key
	//
	// cat ca.crt
	goodCaPem := []byte(`
-----BEGIN CERTIFICATE-----
MIIC+zCCAeOgAwIBAgIJAJgoG8+Po5HbMA0GCSqGSIb3DQEBDQUAMBMxETAPBgNV
BAMMCE15Um9vdENBMCAXDTE5MTAzMTAwMDgxNFoYDzIxMTkxMDA3MDAwODE0WjAT
MREwDwYDVQQDDAhNeVJvb3RDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAMkTo6X7HZFKXEX3qttwM41DIz2X6cy9HK0Qg1L3+JG0GTf293NUpgnH8MdU
6fAUQKG5DPchmCkGia4gx1rE8WOwRoFVMRq13qSxvq8e0iOc+anW4wR02Z96tvLO
coDf0XffX0+0nZ2YV5GoSYfvLe0xLRSxbly6X1JFGe7wbIhQmygkLqM1920toeCr
Jsw45cv4NPGHP9/h7G1GFWGg53DtfiDzuktVKhkgWFWXR52+ENt09w93GY+OZpG3
q+DpiCO0g3eZPo9RxwqB5NNhZGSaUBzuGOjNXoxbGu68Ag0XZP4yGF7nuVbcekCG
8pjNXW7PB2QqsqMwzAOcvteND8kCAwEAAaNQME4wHQYDVR0OBBYEFEROSTYr1PbP
mEyHujn+NIg96YI7MB8GA1UdIwQYMBaAFEROSTYr1PbPmEyHujn+NIg96YI7MAwG
A1UdEwQFMAMBAf8wDQYJKoZIhvcNAQENBQADggEBAA4xfJTAHezqAn/z0sEcBwlx
YgppARHK7ZYMeiAhfEjlIe1jJimv92kPgeTQ6qCsxlhXr+mrABwkSltYvoqkTLcv
8VtDSUw2ajzz6TXdybyCI2oXIxvDSqjo9UKAlbsZXgL/MQ2/vQWpl5cBY51y7DsW
CPuXiHLfUEk8ZvyBYCkzieZOc+HSanuIIrjjeSt/XkMdu12XWEwUKffl/loanYq9
TelzhwvxmGbGZ1MucLtU4i+GCt3QWKwAfUnT8qNZ/jDOaWiOkI6AptoiQEH/Mch/
0pPrY0djLiSe+t7sifLWhhzcQijDa9SUS/58CUQDDFZcI5NNz80fJYFaEU9V97M=
-----END CERTIFICATE-----`)

	// # signed using the good CA
	// openssl req -new -keyout server.key -nodes -newkey rsa:2048 \
	//   -subj "/CN=localhost" \
    //   | openssl x509 -req -CAkey ca.key -CA ca.crt -days 36500 \
	//   -set_serial $RANDOM -sha512 -out server.crt
	//
	// cat server.crt
	certPem := []byte(`
-----BEGIN CERTIFICATE-----
MIICnjCCAYYCAlOjMA0GCSqGSIb3DQEBDQUAMBMxETAPBgNVBAMMCE15Um9vdENB
MCAXDTE5MTAzMTAwMDgxOFoYDzIxMTkxMDA3MDAwODE4WjAUMRIwEAYDVQQDDAls
b2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCr/3U2M5bP
VrJCfTbfW1HzPKVH8QNrQhVkpWX2ZQt8fNGC7dJXN3QUSKBkXUo7GbuIUBoE+GpV
PmNGFYbj0wm2Gju0gPbsTIIH504+IG1OyBhc/6EJ34KZvGKr1mx/rpNIAp/A+Cxa
CQxJuwk/61LKPbSKJ+6Fk5HpMmLEAuXCmvnSjy0GtvQuVm7wM1aZW9w0HDTa1/Xs
TxqOWC9qGby6Mv8Pww8iiDCrqzq7LN0IumL41MPXYbaPfk5EyaUNbn3bHFQACB2E
pqR8HR8V/GjYEjXrhbbscRyOfkYopk8htBQEX8D05WIGkIZpCbuYKXTJs2I3YAE1
55ewWA71CB0pAgMBAAEwDQYJKoZIhvcNAQENBQADggEBAMYoLa61QAcTQgFFWVFG
4MPbdpfIeRh7VPxyV4vE0eeh+KG8Ak0J91lApwnP2jfOBiygoqJJwA2W1xTSjduu
L1A/mkm221ITPJsJmUidJHegJXH0zdFeJIDYK4Ddmi/l8/c/cDWr47laFDPOQH9w
d7zw43/7c2s/daOXDTF5+EHwUgO6X8wpQmVHzi2y6tbtpUtXXNyWstmsglHndf9u
rK6sUCvJtdte6phAicgSHwj/JHz4qmfWlN1yT3TJwKB2Ji8rsmAEhoazR+t+oghN
UkngVP8q/Afj/DIm16ybd2sM0AtLxFMgpYUU+bxQOO7TSE0joxNFFWA9pQYsg5xq
KMY=
-----END CERTIFICATE-----`)

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(goodCaPem)

	verifyPeerCertificate := buildVerifyPeerCertificateNoHostCheck(caCertPool)

	block, _ := pem.Decode(certPem)
	err := verifyPeerCertificate([][]byte{block.Bytes}, nil)

	// We should not get an error because certPem is signed by good CA
	assert.NoError(t, err)
}

func Test_buildVerifyPeerCertificateNoHostCheck_RejectsBadCert(t *testing.T) {
	// openssl req -extensions v3_ca -new -x509 -days 36500 -nodes \
	//   -subj "/CN=MyEvilCA" -newkey rsa:2048 -sha512 -out evil_ca.crt \
	//   -keyout evil_ca.key
	//
	// cat evil_ca.crt
	badCaPem := []byte(`
-----BEGIN CERTIFICATE-----
MIIC+zCCAeOgAwIBAgIJAOtB5xG8ve64MA0GCSqGSIb3DQEBDQUAMBMxETAPBgNV
BAMMCE15RXZpbENBMCAXDTE5MTAzMTAwMDgxNloYDzIxMTkxMDA3MDAwODE2WjAT
MREwDwYDVQQDDAhNeUV2aWxDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAMfbA3/kuUmRuhWUHgGNHHcPzD4HwLgAC8/pa8MM6TXVwiACNCSiCUqAfZ/M
sgO+EKzH++dW0WFRhsygMZOV1GQbLJSJ+MdxvUVKpsAMTAYgKjgJfucXW9h5IHZQ
n0SWkP0J8XBIuAlXmILbY9aQhOJHws9Z98LdXXD8mRfHuVguB9znmt6tpoYLeVQH
LYNELEaLYL6Il0vGDdyoSz8Txz1uPBDgbrWReWYWydFg4G3ZdjcRsO5P3BDgXjMq
5MryuHOI/MuPKMUYR+RWAFD4OajvpiXF2uj2nbokq1nTMVrE4m8FNcm9rrTkme3l
trlNrQCM2qpSKyNvlKNjQQSAEOkCAwEAAaNQME4wHQYDVR0OBBYEFI6tfMwnVUY1
tlusJEzVHzhAMg4QMB8GA1UdIwQYMBaAFI6tfMwnVUY1tlusJEzVHzhAMg4QMAwG
A1UdEwQFMAMBAf8wDQYJKoZIhvcNAQENBQADggEBAC5kQKsCTMfmOgHOFvH5RQlo
RlsC41dzclnj1+9lZfI8OdksEwP7Z78yMcY+MiVVfmzqn62Vdo6Lkdvj37YlMRju
y2bx7ocBXbL8XODyFAPJGeH1JMyMwEvlhD49+4d1AiJ6CqU5C4Rk0Q/ZShd5jWQm
ITqnaOOJF7ou59VEwCAZj0AUPyD1yoIOQQ6ri2TT+/zvUlQW13JV9A++4wuRz19i
XjyN8GJEM3kNn+uTY5K9QbQqnbPJUldjj4ac+mJcUdn5V1rhGBemgat9vXhnFr02
eg3SJczyxVyceQ8h1AtAmHIJwxoXmaJoh9dK0MbCU/C9Nud3xXUim/TMkJeK4vc=
-----END CERTIFICATE-----`)

	// # not signed using bad CA
	// openssl req -new -keyout server.key -nodes -newkey rsa:2048 \
	//   -subj "/CN=localhost" \
	//   | openssl x509 -req -CAkey ca.key -CA ca.crt -days 36500 \
	//   -set_serial $RANDOM -sha512 -out server.crt
	//
	// cat server.crt
	certPem := []byte(`
-----BEGIN CERTIFICATE-----
MIICnjCCAYYCAlOjMA0GCSqGSIb3DQEBDQUAMBMxETAPBgNVBAMMCE15Um9vdENB
MCAXDTE5MTAzMTAwMDgxOFoYDzIxMTkxMDA3MDAwODE4WjAUMRIwEAYDVQQDDAls
b2NhbGhvc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCr/3U2M5bP
VrJCfTbfW1HzPKVH8QNrQhVkpWX2ZQt8fNGC7dJXN3QUSKBkXUo7GbuIUBoE+GpV
PmNGFYbj0wm2Gju0gPbsTIIH504+IG1OyBhc/6EJ34KZvGKr1mx/rpNIAp/A+Cxa
CQxJuwk/61LKPbSKJ+6Fk5HpMmLEAuXCmvnSjy0GtvQuVm7wM1aZW9w0HDTa1/Xs
TxqOWC9qGby6Mv8Pww8iiDCrqzq7LN0IumL41MPXYbaPfk5EyaUNbn3bHFQACB2E
pqR8HR8V/GjYEjXrhbbscRyOfkYopk8htBQEX8D05WIGkIZpCbuYKXTJs2I3YAE1
55ewWA71CB0pAgMBAAEwDQYJKoZIhvcNAQENBQADggEBAMYoLa61QAcTQgFFWVFG
4MPbdpfIeRh7VPxyV4vE0eeh+KG8Ak0J91lApwnP2jfOBiygoqJJwA2W1xTSjduu
L1A/mkm221ITPJsJmUidJHegJXH0zdFeJIDYK4Ddmi/l8/c/cDWr47laFDPOQH9w
d7zw43/7c2s/daOXDTF5+EHwUgO6X8wpQmVHzi2y6tbtpUtXXNyWstmsglHndf9u
rK6sUCvJtdte6phAicgSHwj/JHz4qmfWlN1yT3TJwKB2Ji8rsmAEhoazR+t+oghN
UkngVP8q/Afj/DIm16ybd2sM0AtLxFMgpYUU+bxQOO7TSE0joxNFFWA9pQYsg5xq
KMY=
-----END CERTIFICATE-----`)

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(badCaPem)

	verifyPeerCertificate := buildVerifyPeerCertificateNoHostCheck(caCertPool)

	block, _ := pem.Decode(certPem)
	err := verifyPeerCertificate([][]byte{block.Bytes}, nil)

	// We should get an error becase certPem is not signed by bad CA
	assert.Error(t, err)
}
