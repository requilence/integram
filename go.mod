module github.com/requilence/integram

go 1.14

require (
	github.com/dchest/uniuri v0.0.0-20160212164326-8902c56451e9 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/garyburd/redigo v1.6.0 // indirect
	github.com/gin-gonic/gin v1.6.3
	github.com/gomodule/redigo v1.8.1 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/kelseyhightower/envconfig v1.3.0
	github.com/kennygrant/sanitize v1.2.3
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/mrjones/oauth v0.0.0-20170225175752-3f67d9c27435
	github.com/onsi/ginkgo v1.12.3 // indirect
	github.com/requilence/jobs v0.4.1-0.20180308093307-531b5ae549de
	github.com/requilence/telegram-bot-api v4.5.2-0.20190104221209-440431af8b3c+incompatible
	github.com/requilence/url v0.0.0-20180119020412-6fc4fc0c65da
	github.com/sirupsen/logrus v1.0.5
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/technoweenie/multipartstreamer v1.0.1 // indirect
	github.com/throttled/throttled v2.2.4+incompatible
	github.com/vova616/xxhash v0.0.0-20130313230233-f0a9a8b74d48
	github.com/weekface/mgorus v0.0.0-20170606101347-83720e22971a
	golang.org/x/oauth2 v0.0.0-20180402223937-921ae394b943
	google.golang.org/appengine v1.0.0 // indirect
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/mgo.v2 v2.0.0-20160818020120-3f83fa500528
)

//replace github.com/gin-gonic/gin 588879e55f3c13099159e1f24b7b90946f31266b => github.com/requilence/gin v1.1.5-0.20180413113949-588879e55f3c
