Integram 2.0
===========

Framework and platform for integrating services into [Telegram](https://telegram.org) using official [Bot API](https://core.telegram.org/bots/api)

[![CircleCI](https://img.shields.io/circleci/project/requilence/integram.svg)](https://circleci.com/gh/requilence/integram) [![Docker Image](https://img.shields.io/docker/build/integram/integram.svg)](https://hub.docker.com/r/integram/integram/) [![GoDoc](https://godoc.org/github.com/Requilence/integram?status.svg)](https://godoc.org/github.com/requilence/integram)

![Screencast](https://st.integram.org/img/screencast4.gif)

How to use Integram in Telegram
------------------
Just use this links to add bots to your Telegram
* [Trello](https://t.me/trello_bot?start=f_github)
* [Gitlab](https://t.me/gitlab_bot?start=f_github)
* [Bitbucket](https://t.me/bitbucket_bot?start=f_github)
* [Simple webhook bot](https://t.me/bullhorn_bot?start=f_github)

* [GitHub](https://telegram.me/githubbot) – GitHub bot was developed by [Igor Zhukov](https://github.com/zhukov) and it is not the part of Integram

Not found you favorite service? [🤘 Vote for it](https://telegram.me/integram_bot?start=vote)

How to run Integram on your own server
------------------

🐳 Docker way
------------------
- Install **docker** and **docker-compose**: https://docs.docker.com/compose/install/
- Clone the repo:
```bash
   git clone github.com/requilence/integram && cd integram
```
- Check the `docker-compose.yml` file for the required ENV vars for each service
    - E.g. in order to run Trello integration you need to export: 
    	- **INTEGRAM_BASE_URL** – the base URL your host accessible with, e.g. **https://integram.org**
	    - **INTEGRAM_PORT** – if set to 443 Integram will use letsencryprt to automatically fetch the SSL cert for the domain used in **INTEGRAM_BASE_URL**. You can also setup to [use your own certs](https://github.com/requilence/integram/blob/master/HOWTO#use-ssl-cert-files-instead-of-letsencrypt)

	    - **TRELLO_BOT_TOKEN** – bot's token you got from [@BotFather](https://t.me/botfather)
	    - You need to [get your own OAuth credentials from Trello](https://trello.com/app-key)
	    - **TRELLO_OAUTH_ID** – API Key
	    - **TRELLO_OAUTH_SECRET** – OAuth Secret
    
    - For the more detailed info about other services you should check corresponding repo at https://github.com/integram-org
- Now you can run the services:
```bash
   docker-compose up -p integram trello gitlab ## You can specify services you want to run
```
- Now you should be able to see the startup logs in your console and ensure that your bots are working correctly in Telegram.
- Add the `-d` argument to run process in background mode:
```bash
   docker-compose up -d -p integram trello gitlab
   
   ## Check the containers status
   docker ps
   
   ## Fetch logs for main container
   docker logs -f $(docker ps -aqf "name=integram_integram")   
```
- To update Integram to the last version:
```bash
    ## Fetch last version of images
    docker-compose pull integram trello gitlab
    ## Restart containers using the new images
    docker-compose up --no-deps -d integram trello gitlab
```


🛠 Old-school way (No docker)
------------------
- First you need to install all requirements: [Go 1.9+](https://golang.org/doc/install), [Go dep](https://github.com/golang/dep#setup), [MongoDB 3.4+ (for data)](https://docs.mongodb.com/manual/administration/install-community/), [Redis 3.2+ (for jobs queue)](https://redis.io/download)

- Then, using [this template](https://github.com/requilence/integram/blob/master/cmd/single-process-mode/main.go) 
 create the `main.go` file and put it to `src/integram/` inside your prefered working directory (e.g. `/var/integram/src/integram/main.go`)

```bash
    ## set the GOPATH to the absolute path of directory containing 'src' directory that you have created before
    export GOPATH=/var/integram
    
    cd $GOPATH/src/integram
    ## install dependencies
    dep init
```

- Specify required ENV variables – check the [Docker way section](https://github.com/requilence/integram#-docker-way)
- Run it
```bash
    go build integram && ./integram
```

### Dependencies

Dependencies are specified in `Gopkg.toml` and fetched using [Go dep](https://github.com/golang/dep)

Contributing
------------------
Feel free to send PRs. If you want to contribute new service integration, please [create the issue](https://integram.org/issues/new) first. Just to make sure developing is not already in progress.

### Libraries that was using in Integram

* [Telegram Bindings](https://github.com/go-telegram-bot-api/telegram-bot-api)
* [Gin – HTTP router and framework](https://github.com/gin-gonic/gin)
* [Mgo – MongoDB driver](https://github.com/go-mgo/mgo)
* [Jobs – background jobs](https://github.com/albrow/jobs)
* [Logrus – structure logging](https://github.com/sirupsen/logrus)


### License
Code available on GPLV3 [license](https://github.com/requilence/integram/blob/master/LICENSE)

![Analytics](https://ga-beacon.appspot.com/UA-80266491-1/github_readme)
