package helpermethods

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"io/ioutil"
	"net/http"
	"net/url"
	"trading_bot/pkg/logger"
)

type HelperMethods struct {
	logger logger.Interface
}

func New(l logger.Interface) *HelperMethods {
	return &HelperMethods{
		logger: l,
	}
}

func (hm HelperMethods) HttpGet(ctx context.Context, url *url.URL, contentType string, webHeaders map[string]string) (string, int, error) {
	if len(contentType) == 0 {
		contentType = "application/x-www-form-urlencoded"
	}

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		hm.logger.Error(err)
		return "", 500, err
	}

	req = req.WithContext(ctx)

	req.Header.Set("Content-Type", contentType)

	// appending headers
	if len(webHeaders) > 0 {
		for key, val := range webHeaders {
			req.Header.Set(key, val)
		}
	}

	// appending to existing query args
	//q := req.URL.Query()
	// q.Add("foo", "bar")

	// assign encoded query string to http request
	//req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		hm.logger.Error(err)
		return "", 500, err
	}

	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		hm.logger.Error(err)
		return "", 500, err
	}

	return string(responseBody), resp.StatusCode, nil
}

func (hm HelperMethods) HttpPost(ctx context.Context, url *url.URL, data []byte, contentType string, webHeaders map[string]string) (string, int, error) {
	if len(contentType) == 0 {
		contentType = "application/x-www-form-urlencoded"
	}

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(data))
	if err != nil {
		hm.logger.Error(err)
		return "", 500, err
	}

	req = req.WithContext(ctx)

	req.Header.Set("Content-Type", contentType)

	// appending headers
	if len(webHeaders) > 0 {
		for key, val := range webHeaders {
			req.Header.Set(key, val)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		hm.logger.Error(err)
		return "", 500, err
	}

	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		hm.logger.Error(err)
		return "", 500, err
	}

	return string(responseBody), resp.StatusCode, nil
}

func (hm HelperMethods) HmacSha512(keyBytes []byte, messageBytes []byte) ([]byte, error) {
	// Create a new HMAC by defining the hash type and the key (as byte array)
	h := hmac.New(sha512.New, keyBytes)

	// Write Data to it
	h.Write(messageBytes)

	return h.Sum(nil), nil
}
