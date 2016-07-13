Integram 2.0
===========

Framework and platform for integrating services into [Telegram](https://telegram.org) using official [Bot API](https://core.telegram.org/bots/api)

[![GoDoc](https://godoc.org/github.com/Requilence/integram?status.svg)](https://godoc.org/github.com/Requilence/integram) [![CircleCI](https://img.shields.io/circleci/project/Requilence/integram.svg)](https://circleci.com/gh/Requilence/integram)

![Screencast](https://1153359166.rsc.cdn77.org/integram/img/screencast4.gif)

How to use Integram
------------------
Just use this links to add integrations you are interested in
* [Trello](https://telegram.me/trello_bot?start=f_github)
* [Gitlab](https://telegram.me/gitlab_bot?start=f_github)
* [Bitbucket](https://telegram.me/bitbucket_bot?start=f_github)
* [Simple webhook bot](https://telegram.me/Bullhorn_bot?start=f_github)

* [GitHub](https://telegram.me/githubbot) – GitHub bot was developed by [Igor Zhukov](https://github.com/zhukov) and it is not the part of Integram

Not found you favorite service? [🤘 Vote for it](https://telegram.me/integram_bot?start=vote)
Running Integram on your side
------------------
You can run Integram on your own server. 
- Create the **main.go** file (example is below)
- Use your own bot created with [Botfather](https://telegram.me/botfather).
- For the each service you are want to use you need to create an OAuth client(application) in it
- Set environment variable **GOPATH** to the directory contains **main.go** file
- Run **go get github.com/requilence/integram**
- Specify environment variables:
    - **INTEGRAM_PORT** - if set to 443, integram.crt and integram.key must be presented in the root
    - **INTEGRAM_BASE_URL** - the base URL the host accessible with, f.e. **https://integram.org**
- Run **go run main.go** or **go build && ./integram**

main.go example
------------------
```go
package main

import (
	"github.com/requilence/integram"
	"github.com/requilence/integram/services/trello"
	"github.com/requilence/integram/services/gitlab"
)

func main() {
	integram.Debug=true
	
	integram.Register(
        trello.Config{
            integram.OAuthProvider{
                ID:     "TRELLO_APP_KEY",
                Secret: "TRELLO_APP_SECRET",
            },
        },
        "BOT_TOKEN_PROVIDED_BY_@BOTFATHER",
    )

    integram.Register(
        gitlab.Config{
            integram.OAuthProvider{
                ID:     "GITLAB_APP_ID",
                Secret: "GITLAB_APP_SECRET",
            },
        },
        "BOT_TOKEN_PROVIDED_BY_@BOTFATHER",
    )

		
	integram.Run()
}
```

### Dependencies and vendor directory 

All dependencies come with package itself and may be modified directly (see the [Third party libraries](https://github.com/Requilence/integram#third-party-libraries))

### Requirements

Go 1.5+, MongoDB 3.2+ (for data), Redis 3.2.0+ (for jobs queue)

Contributing
------------------
Feel free to send PRs. If you want to contribute new service integration, please [create the issue](https://integram.org/issues/new) first. Just to make sure developing is not already in progress.

### Third party libraries

* [Telegram Bindings](https://github.com/go-telegram-bot-api/telegram-bot-api) *
* [Gin – HTTP router and framework](https://github.com/gin-gonic/gin)
* [Mgo – MongoDB driver](https://github.com/go-mgo/mgo)
* [Jobs – background jobs](https://github.com/albrow/jobs) *
* [Redigo – Redis driver](https://github.com/garyburd/redigo/redis)
* [Logrus – structure logging](https://github.com/Sirupsen/logrus)
* [Trello bindings](https://github.com/hackerlist/trello) *
* [Gitlab bindings](https://github.com/xanzy/go-gitlab) * 
* [Bitbucket bindings](https://github.com/ktrysmt/go-bitbucket) *

\* - **package source is modified**

### License
Code available on GPLV3 [license](https://github.com/requilence/integram/blob/master/LICENSE)

![Analytics](https://ga-beacon.appspot.com/UA-80266491-1/github_readme)
