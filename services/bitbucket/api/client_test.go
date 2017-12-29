package bitbucket

import (
	"testing"
)

func TestBasicAuth(t *testing.T) {

	c := NewBasicAuth("example", "password")

	if c.Auth.user != "example" {
		t.Error("username is not equal")
	}

	if c.Auth.password != "password" {
		t.Error("password is not equal")
	}
}

func TestOAuth(t *testing.T) {

	c := NewOAuth("aaaaaaaa", "11111111111111")

	if c.Auth.user != "aaaaaaaa" {
		t.Error("app_id is not equal")
	}

	if c.Auth.password != "11111111111111" {
		t.Error("secret is not equal")
	}
}
