package trello

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const trellourl = "https://api.trello.com/1/"

type Client struct {
	apikey    string
	apisecret string
	apitoken  string
}

func IsBadToken(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasSuffix(err.Error(), "invalid token")
}

func New(key, secret, token string) *Client {
	return &Client{key, secret, token}
}

func (c *Client) RequestWithHeaders(method, function string, postbody io.Reader, headers map[string]string, extra url.Values) ([]byte, error) {
	postdata := url.Values{"key": {c.apikey}, "token": {c.apitoken}}
	for k, v := range extra {
		postdata[k] = v
	}
	url := trellourl + function + "?" + postdata.Encode()
	req, err := http.NewRequest(method, url, postbody)
	for key, val := range headers {
		req.Header.Add(key, val)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode > 300 {
		return nil, fmt.Errorf("Trello returned code %s for %s %s: %s", resp.Status, method, url, body)
	}

	return body, nil
}

func (c *Client) Request(method, function string, postbody io.Reader, extra url.Values) ([]byte, error) {
	return c.RequestWithHeaders(method, function, postbody, nil, extra)
}

func getfield(js []byte, field string) (string, error) {
	var i interface{}
	err := json.Unmarshal(js, &i)
	if err != nil {
		return "", err
	}

	ma, ok := i.(map[string]interface{})

	if !ok {
		return "", fmt.Errorf("json not a dictionary")
	}

	f, ok := ma[field]
	if !ok {
		return "", fmt.Errorf("no field %s", field)
	}

	str, ok := f.(string)

	if !ok {
		return "", fmt.Errorf("field %s not a string", field)
	}

	return str, nil
}
