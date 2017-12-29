package main

import (
	"github.com/requilence/integram"
	"github.com/requilence/integram/services/trello"
	"github.com/kelseyhightower/envconfig"
)

func main(){
	var cfg trello.Config
	envconfig.MustProcess("TRELLO", &cfg)

	integram.Register(
		cfg,
		cfg.BotConfig.Token, //hx_gitlab_bot,
	)

	integram.Run()
}