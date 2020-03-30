module github.com/datastax/cass-operator

go 1.13

require (
	github.com/datastax/cass-operator/mage v0.0.0-00010101000000-000000000000
	github.com/emicklei/go-restful-swagger12 v0.0.0-20170926063155-7524189396c6 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/gorilla/schema v1.1.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/inconshreveable/go-vhost v0.0.0-20160627193104-06d84117953b // indirect
	github.com/jessevdk/go-flags v1.4.0 // indirect
	github.com/magefile/mage v1.9.0
	github.com/mjibson/appstats v0.0.0-20151004071057-0542d5f0e87e // indirect
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/rogpeppe/go-charset v0.0.0-20190617161244-0dc95cdf6f31 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	gopkg.in/vmihailenco/msgpack.v2 v2.9.1 // indirect
)

replace github.com/datastax/cass-operator/mage => ./mage

replace github.com/datastax/cass-operator/operator => ./operator
