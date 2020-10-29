Integram 2.0
===========

Framework and platform to integrate services with [Telegram](https://telegram.org) using the official [Telegram Bot API](https://core.telegram.org/bots/api)

ℹ️ Individual integration repos are located at https://github.com/integram-org.

[![CircleCI](https://img.shields.io/circleci/project/requilence/integram.svg)](https://circleci.com/gh/requilence/integram) [![Docker Image](https://img.shields.io/docker/build/integram/integram.svg)](https://hub.docker.com/r/integram/integram/) [![GoDoc](https://godoc.org/github.com/Requilence/integram?status.svg)](https://godoc.org/github.com/requilence/integram)

![Screencast](https://st.integram.org/img/screencast4.gif)

How to use Integram in Telegram (using public bots)
------------------
Just use these links to add bots to your Telegram
* [Trello](https://t.me/trello_bot?start=f_github)
* [Gitlab](https://t.me/gitlab_bot?start=f_github)
* [Bitbucket](https://t.me/bitbucket_bot?start=f_github)
* [Simple webhook bot](https://t.me/bullhorn_bot?start=f_github)

* [GitHub](https://telegram.me/githubbot) – GitHub bot was developed by [Igor Zhukov](https://github.com/zhukov) and it is not part of Integram

Did not find your favorite service? [🤘 Vote for it](https://telegram.me/integram_bot?start=vote)

How to host Integram on your own server (using your private bots)
------------------

🐳 Docker way
------------------
- Prerequisites :
    - You will need [docker](https://docs.docker.com/install/) and [docker-compose](https://docs.docker.com/compose/install/) installed
    - Create your Telegram bot(s) by talking to [@BotFather](https://t.me/botfather)
- Clone the repo:
```bash
   git clone https://github.com/requilence/integram && cd integram
```
- Check the `docker-compose.yml` file for the required ENV vars for each service
    - E.g. in order to run the Trello integration you will need to export: 
    	- **INTEGRAM_BASE_URL** – the base URL where your Integram host will be accessible, e.g. **https://integram.org**
	    - **INTEGRAM_PORT** – if set to 443 Integram will use ssl.key/ssl.cert at /go/.conf.
	    	- For **Let's Encrypt**: `ssl.cert` has to be `fullchain.pem`, not `cert.pem`
	    
	        This directory is mounted on your host machine. Just get the path and put these files inside
            ```bash
               ## Get the path of config directory on the host machine
               docker volume inspect -f '{{ .Mountpoint }}' integram_data-mainapp
            ```
	    - **TRELLO_BOT_TOKEN** – your bot's token you got from [@BotFather](https://t.me/botfather)
	    - You will need to [get your own OAuth credentials from Trello](https://trello.com/app-key)
	      - **TRELLO_OAUTH_ID** – API Key
	      - **TRELLO_OAUTH_SECRET** – OAuth Secret
    
    - For more detailed info about other services you should check the corresponding repo at https://github.com/integram-org
- Export the variables you identified in the previous step, for instance on linux this should be something like:
```bash
   export INTEGRAM_PORT=xxxx
   export ...
```
- Now you can run the services (linux: careful if you need to sudo this, the exports you just did will not be available) :
```bash
   docker-compose -p integram up trello gitlab ## Here you specify the services you want to run
```
- Or in background mode (add `-d`):
```bash
   docker-compose -p integram up -d trello gitlab
```
- You should now see Integram's startup logs in your console
- In Telegram, you can now start your bots (`/start`) and follow their directions, configure them using `/settings`
- Some useful commands:
```bash
   ## Check the containers status
   docker ps
   
   ## Fetch logs for main container
   docker logs -f $(docker ps -aqf "name=integram_integram")   
```
- To update Integram to the latest version:
```bash
    ## Fetch last version of images
    docker-compose pull integram trello gitlab
    ## Restart containers using the new images
    docker-compose -p integram up -d trello gitlab
```


🛠 Old-school way (No docker)
------------------
- First you need to install all requirements: [Go 1.9+](https://golang.org/doc/install), [Go dep](https://github.com/golang/dep#setup), [MongoDB 3.4+ (for data)](https://docs.mongodb.com/manual/administration/install-community/), [Redis 3.2+ (for jobs queue)](https://redis.io/download)

- Then, using [this template](https://github.com/requilence/integram/blob/master/cmd/single-process-mode/main.go) 
 create the `main.go` file and put it in `src/integram/` inside your preferred working directory (e.g. `/var/integram/src/integram/main.go`)

```bash
    ## set the GOPATH to the absolute path of directory containing 'src' directory that you have created before
    export GOPATH=/var/integram
    
    cd $GOPATH/src/integram
    ## install dependencies
    dep init
```

- Specify the required ENV variables – check the [Docker way section](https://github.com/requilence/integram#-docker-way)
- Run it
```bash
    go build integram && ./integram
```

### Dependencies

Dependencies are specified in `Gopkg.toml` and fetched using [Go dep](https://github.com/golang/dep)

Contributing
------------------
Feel free to send PRs. If you want to contribute new service integrations, please [create an issue](https://integram.org/issues/new) first. Just to make sure someone is not already working on it.

### Libraries used in Integram

* [Telegram Bindings](https://github.com/go-telegram-bot-api/telegram-bot-api)
* [Gin – HTTP router and framework](https://github.com/gin-gonic/gin)
* [Mgo – MongoDB driver](https://github.com/go-mgo/mgo)
* [Jobs – background jobs](https://github.com/albrow/jobs)
* [Logrus – structure logging](https://github.com/sirupsen/logrus)


### License
Code licensed under GPLV3 [license](https://github.com/requilence/integram/blob/master/LICENSE)

![Analytics](https://ga-beacon.appspot.com/UA-80266491-1/github_readme)
