package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

const baseURL = "https://ghcr.io/v2/"

func GHCRImage(repo, image, tag string) string {
	return fmt.Sprintf("ghcr.io/%s/%s:%s", repo, image, tag)
}

func NewGHCRClient(client *http.Client, token string) *GHCRClient {
	return &GHCRClient{
		token:  base64.StdEncoding.EncodeToString([]byte(token)),
		client: client,
	}
}

type GHCRClient struct {
	token  string
	client *http.Client
}

var ErrNotFound = errors.New("not found")

var errHttpResponse = errors.New("error response")

func (gc *GHCRClient) ListTags(ctx context.Context, image ...string) ([]string, error) {
	path := append(image, "tags", "list")

	var tagResponse struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	res, err := gc.get(ctx, &tagResponse, path...)
	if errors.Is(err, errHttpResponse) {
		if res.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("error response from server: %s", res.Status)
	} else if err != nil {
		return nil, err
	}

	return tagResponse.Tags, nil
}

func (gc *GHCRClient) get(ctx context.Context, o any, path ...string) (_ *http.Response, outErr error) {
	resURL, err := url.JoinPath(baseURL, path...)

	req, err := http.NewRequestWithContext(ctx, "GET", resURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+gc.token)

	res, err := gc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			outErr = errors.Join(outErr, err)
		}
	}()

	if res.StatusCode != http.StatusOK {
		return res, errHttpResponse
	}

	dec := json.NewDecoder(res.Body)

	err = dec.Decode(o)
	if err != nil {
		return res, fmt.Errorf("parse response: %w", err)
	}

	return res, nil
}
