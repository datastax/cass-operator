// Copyright DataStax, Inc.
// Please see the included license file for details.

package httphelper

import "net/http"

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}