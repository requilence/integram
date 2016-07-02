package decent

import (
	"fmt"
	"math/rand"
	"time"
)

type format string

func Shuffle(formats ...string) format {
	l := len(formats)

	rand.Seed(time.Now().UnixNano())
	i := rand.Intn(l)
	return format(formats[i])
}

func (f format) S(params ...interface{}) string {
	return fmt.Sprintf(string(f), params...)
}
