// Package url created to provide Marshaling and Unmarshaling for url.URL
package url

import (
	"bytes"
	"fmt"
	nativeurl "net/url"
	"strings"
)

// URL is native url.URL struct
type URL nativeurl.URL

// Parse parses rawurl into a URL structure.
// The rawurl may be relative or absolute.
func Parse(rawurl string) (url *URL, err error) {
	un, err := nativeurl.Parse(rawurl)
	u := URL(*un)
	return &u, err
}

// UnmarshalText calls url.Parse
func (u *URL) UnmarshalText(p []byte) error {
	nu, err := nativeurl.Parse(string(p))
	if err != nil {
		return err
	}
	(*u) = URL(*nu)
	return nil
}

// UnmarshalBinary calls url.Parse
func (u *URL) UnmarshalBinary(p []byte) error {
	nu, err := nativeurl.Parse(string(p))
	if err != nil {
		return err
	}
	(*u) = URL(*nu)
	return nil
}

// MarshalText just calls String()
func (u *URL) MarshalText() ([]byte, error) {
	return []byte(u.String()), nil
}

// MarshalBinary just calls String()
func (u *URL) MarshalBinary() ([]byte, error) {
	return []byte(u.String()), nil
}

// UnmarshalJSON parses JSON string into url.URL
func (u *URL) UnmarshalJSON(p []byte) error {
	nu, err := nativeurl.Parse(string(bytes.Trim(p, `"`)))
	if err != nil {
		return err
	}
	(*u) = URL(*nu)
	return nil
}

// MarshalJSON turns url into a JSON string
func (u *URL) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf(`"%s"`, u.String())
	return []byte(s), nil
}

// GetPath returns url.Path with leading '/' removed
func (u *URL) GetPath() string {
	if u == nil {
		return ""
	}
	return strings.TrimLeft(u.Path, "/")
}

// GetHost url.Host without the port suffix
func (u *URL) GetHost() string {
	if u == nil {
		return ""
	}
	i := strings.Index(u.Host, ":")
	if i == -1 {
		return u.Host
	}
	return u.Host[0:i]
}

// String returns the string representation
func (u *URL) String() string {
	return (*nativeurl.URL)(u).String()
}
