package decent

import (
	"fmt"
	"time"
)

// Relative returns relative decent time
func Relative(target time.Time) string {
	now := time.Now()
	day := ""

	if target.Year() == now.Year() {
		days := target.YearDay() - now.YearDay()
		switch days {
		case -1:
			day = "Yesterday, " + target.Weekday().String()
		case 0:
			day = "Today, " + target.Weekday().String()
		case 1:
			day = "Tomorrow, " + target.Weekday().String()
		default:
			if days > 0 && days < 7 && (days < 6 || target.Weekday() != now.Weekday()) {
				day = target.Weekday().String()
			}
		}
	}

	if day == "" {
		day = target.Format("2 January")
		return fmt.Sprintf("%s", day)
	}
	tz := target.Format("-07:00")
	if tz[3:] == ":00" {
		tz = tz[0:3]
	}
	if tz[1:2] == "0" {
		tz = tz[0:1] + tz[2:]
	}
	return fmt.Sprintf("%s, %s (UTC%s)", day, target.Format("15:04"), tz)
}
