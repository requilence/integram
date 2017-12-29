package main

import (
	"github.com/requilence/integram"
	"github.com/requilence/integram/services/webhook"
	"github.com/kelseyhightower/envconfig"
)

func main(){
	var cfg webhook.Config
	envconfig.MustProcess("WEBHOOK", &cfg)

	integram.Register(
		cfg,
		cfg.BotConfig.Token,
	)

	integram.Run()
}