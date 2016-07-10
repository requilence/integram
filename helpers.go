package integram

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/requilence/integram/url"
	"github.com/vova616/xxhash"
	"io/ioutil"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"time"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-0123456789"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
)

var (
	dunno       = []byte("???")
	centerDot   = []byte("·")
	dot         = []byte(".")
	slash       = []byte("/")
	tzLocations = make(map[string]*time.Location)
)

func tzLocation(name string) *time.Location {
	if name == "" {
		name = "UTC"
	}
	if val, ok := tzLocations[name]; ok {
		if val == nil {
			return tzLocation("UTC")
		}
		return val
	}
	l, err := time.LoadLocation(name)
	tzLocations[name] = l

	if err != nil || l == nil {
		log.WithField("tz", name).Error("Can't find TZ")
		return tzLocation("UTC")
	}

	return l
}

// source returns a space-trimmed slice of the n'th line.
func source(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.TrimSpace(lines[n])
}

// function returns, if possible, the name of the function containing the PC.
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contains dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastslash := bytes.LastIndex(name, slash); lastslash >= 0 {
		name = name[lastslash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}

// stack returns a nicely formated stack frame, skipping skip frames
func stack(skip int) []byte {
	buf := new(bytes.Buffer) // the returned data
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	var lines [][]byte
	var lastFile string
	for i := skip; ; i++ { // Skip the expected number of frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
		if file != lastFile {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}
	return buf.Bytes()
}

func randomInRange(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}

// SliceContainsString returns true if []string contains string
func SliceContainsString(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
func compactHash(s string) string {
	a := md5.Sum([]byte(s))
	return base64.RawURLEncoding.EncodeToString(a[:])
}

func checksumString(s string) string {
	b := make([]byte, 4)
	cs := xxhash.Checksum32([]byte(s))

	binary.LittleEndian.PutUint32(b, cs)
	return base64.RawURLEncoding.EncodeToString(b)
}

func randString(n int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	b := make([]byte, n)
	for i := 0; i < n; {
		if idx := int(rand.Int63() & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i++
		}
	}
	return string(b)
}

// URLMustParse returns url.URL from static string. Don't use it with a dynamic param
func URLMustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		log.Errorf("Expected URL to parse: %q, got error: %v", s, err)
	}
	return u
}

func getFuncName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func getBaseURL(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
}
func getHostFromURL(s string) string {
	h := strings.SplitAfterN(s, "://", 2)
	if len(h) > 1 {
		m := strings.SplitN(h[1], "/", 2)
		return m[0]
	}

	return ""
}
