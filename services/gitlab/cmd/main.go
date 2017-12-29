package main

import (
	"github.com/requilence/integram"
	"github.com/requilence/integram/services/gitlab"
	"github.com/kelseyhightower/envconfig"
)

func main(){
	var cfg gitlab.Config
	envconfig.MustProcess("GITLAB", &cfg)

	integram.Register(
		cfg,
		cfg.BotConfig.Token,
	)

	integram.Run()
}