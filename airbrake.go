package main

import (
    "errors"

    "github.com/airbrake/gobrake"
)


var airbrake = gobrake.NewNotifier(168852, "427e36966903c867165c950c73fa3301")


func init() {
    airbrake.AddFilter(func(notice *gobrake.Notice) *gobrake.Notice {
        notice.Context["environment"] = "production"
        return notice
    })
}

func main() {
    defer airbrake.Close()
    defer airbrake.NotifyOnPanic()

    airbrake.Notify(errors.New("operation failed"), nil)
}