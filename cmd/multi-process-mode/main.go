// This entrypoint will run only the main process of Integram
// It's intended to:
//
// – route incoming webhooks to corresponding services
// - resolve webpreviews
// – send outgoing Telegram messages
//
// You should run services in the separate processes

package main

import "github.com/requilence/integram"


func main(){
	integram.Run()
}

