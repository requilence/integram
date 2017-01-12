package integram

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/requilence/integram/url"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	format = "Jan 2 15:04:05"
)

func Papertrail() {

	PThost := os.Getenv("PAPERTRAIL_HOST")
	if PThost != "" {
		PTport, _ := strconv.Atoi(os.Getenv("PAPERTRAIL_PORT"))
		baseURL := os.Getenv("INTEGRAM_BASE_URL")
		host := "integram.org"
		u, _ := url.Parse(baseURL)
		if u != nil {
			host = u.Host
		}
		hook, err := newPapertrailHook(&hook{
			Host:     PThost,
			Port:     PTport,
			Hostname: host,
			Appname:  "integram",
		})

		if err != nil {
			panic(err)
		}
		log.AddHook(hook)
	}
}

// PapertrailHook to send logs to a logging service compatible with the Papertrail API.
type hook struct {
	// Connection Details
	Host string
	Port int

	// App Details
	Appname  string
	Hostname string

	udpConn net.Conn
}

// NewPapertrailHook creates a hook to be added to an instance of logger.
func newPapertrailHook(hook *hook) (*hook, error) {
	var err error

	hook.udpConn, err = net.Dial("udp", fmt.Sprintf("%s:%d", hook.Host, hook.Port))
	return hook, err
}

// Fire is called when a log event is fired.
func (hook *hook) Fire(entry *log.Entry) error {
	date := time.Now().Format(format)
	msg, _ := entry.String()
	payload := fmt.Sprintf("<22> %s %s %s: %s", date, hook.Hostname, hook.Appname, msg)

	bytesWritten, err := hook.udpConn.Write([]byte(payload))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to send log line to Papertrail via UDP. Wrote %d bytes before error: %v", bytesWritten, err)
		return err
	}

	return nil
}

// Levels returns the available logging levels.
func (hook *hook) Levels() []log.Level {
	return []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		//	logrus.WarnLevel,
		//	logrus.InfoLevel,
		//	logrus.DebugLevel,
	}
}
