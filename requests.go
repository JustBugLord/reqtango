package reqtango

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"sort"

	"github.com/fereidani/httpdecompressor"
)

type Response struct {
	Status     string
	StatusCode int
	Body       string
}

type RequestBuilder struct {
	DefaultHeaders map[string]string
	*http.Client
}

func NewRequestBuilderSimple() *RequestBuilder {
	return &RequestBuilder{
		Client: http.DefaultClient,
	}
}

func NewRequestBuilder(defaultHeaders map[string]string) *RequestBuilder {
	return &RequestBuilder{
		defaultHeaders,
		http.DefaultClient,
	}
}

func (b *RequestBuilder) AddHeaders(headers map[string]string) {
	for k, v := range headers {
		b.DefaultHeaders[k] = v
	}
}

func (b *RequestBuilder) Get(url string, headers ...interface{}) (*Response, error) {
	return b.SendRequest("GET", url, nil, headers...)
}

func (b *RequestBuilder) Post(url, body string, headers ...interface{}) (*Response, error) {
	return b.SendRequest("POST", url, bytes.NewBuffer([]byte(body)), headers...)
}

func (b *RequestBuilder) SendRequest(method, url string, body io.Reader, headers ...interface{}) (*Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, errors.New("fail request build: " + err.Error())
	}
	b.formRequestHeaders(req, headers)
	resp, err := b.Do(req)
	if err != nil {
		return nil, errors.New("fail request send: " + err.Error())
	}

	reader, err := httpdecompressor.Reader(resp)
	if err != nil {
		return nil, errors.New("fail request decompress: " + err.Error())
	}
	bodyResponse, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.New("fail response body read: " + err.Error())
	}
	return &Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Body:       string(bodyResponse),
	}, nil
}

func (b *RequestBuilder) formRequestHeaders(request *http.Request, headers ...interface{}) {
	for key, value := range b.DefaultHeaders {
		request.Header.Set(key, value)
	}
	b.appendFields(request, headers)
}

func (b *RequestBuilder) appendFields(request *http.Request, headers interface{}) {
	switch fields := headers.(type) {
	case []interface{}:
		if n := len(fields); n&0x1 == 1 {
			switch subFields := fields[0].(type) {
			case []interface{}:
				fields = subFields
			default:
				fields = fields[:n-1]
			}
		}
		for i := 0; i < len(fields); i += 2 {
			request.Header.Set(fields[i].(string), fields[i+1].(string))
		}
	case map[string]string:
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		kv := make([]interface{}, 2)
		for _, key := range keys {
			kv[0], kv[1] = key, fields[key]
			request.Header.Set(key, kv[0].(string))
		}
	}
}
