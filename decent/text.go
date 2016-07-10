package decent

import (
	"fmt"
	"math/rand"
	"time"
)

// Format is a string with ability to provide params to its placeholders
type Format string

// Shuffle select the random variant from provided strings
func Shuffle(formats ...string) Format {
	l := len(formats)

	rand.Seed(time.Now().UnixNano())
	i := rand.Intn(l)
	return Format(formats[i])
}

// S used to provide params to placeholders
func (f Format) S(params ...interface{}) string {
	return fmt.Sprintf(string(f), params...)
}
