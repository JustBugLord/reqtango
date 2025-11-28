package reqtango

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"

	"github.com/fereidani/httpdecompressor"
)

type Response struct {
	Status     string
	StatusCode int
	Body       []byte
}

type RequestBuilder struct {
	DefaultHeaders map[string]string
	*http.Client
}

func NewRequestBuilderSimple() *RequestBuilder {
	return NewRequestBuilder(nil)
}

func NewRequestBuilder(defaultHeaders map[string]string) *RequestBuilder {
	if defaultHeaders == nil {
		defaultHeaders = make(map[string]string)
	}
	return &RequestBuilder{
		defaultHeaders,
		http.DefaultClient,
	}
}

func (b *RequestBuilder) SetHeaders(headers map[string]string) {
	if headers == nil {
		return
	}
	local := b.DefaultHeaders
	for k, v := range headers {
		local[k] = v
	}
	b.DefaultHeaders = local
}

func (b *RequestBuilder) Get(url string, headers ...interface{}) (*Response, error) {
	return b.SendRequest(GET, url, nil, headers...)
}

func (b *RequestBuilder) GetToStruct(url string, to any, headers ...interface{}) ([]byte, error) {
	return b.SendRequestToStruct(GET, url, nil, to, headers...)
}

func (b *RequestBuilder) Post(url, body string, headers ...interface{}) (*Response, error) {
	return b.SendRequest(POST, url, bytes.NewBuffer([]byte(body)), headers...)
}

func (b *RequestBuilder) PostToStruct(url, body string, to any, headers ...interface{}) ([]byte, error) {
	return b.SendRequestToStruct(POST, url, bytes.NewBuffer([]byte(body)), to, headers...)
}

func (b *RequestBuilder) SendRequest(method Method, url string, body io.Reader, headers ...interface{}) (*Response, error) {
	resp, bodyResp, err := b.SendRequestRaw(method, url, body, headers...)
	if err != nil {
		return nil, err
	}
	return &Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Body:       bodyResp,
	}, nil
}

func (b *RequestBuilder) SendRequestToStruct(method Method, url string, body io.Reader, to any, headers ...interface{}) ([]byte, error) {
	_, data, err := b.SendRequestRaw(method, url, body, headers...)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, to); err != nil {
		return nil, errors.New("fail unmarshal response: " + err.Error())
	}
	return data, nil
}

func (b *RequestBuilder) SendRequestRaw(method Method, url string, body io.Reader, headers ...interface{}) (*http.Response, []byte, error) {
	req, err := http.NewRequest(string(method), url, body)
	if err != nil {
		return nil, nil, errors.New("fail request build: " + err.Error())
	}
	b.formRequestHeaders(req, headers)
	resp, err := b.Do(req)
	if err != nil {
		return nil, nil, errors.New("fail request send: " + err.Error())
	}
	defer resp.Body.Close()

	reader, err := httpdecompressor.Reader(resp)
	if err != nil {
		return nil, nil, errors.New("fail request decompress: " + err.Error())
	}
	defer reader.Close()
	bodyResponse, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, errors.New("fail response body read: " + err.Error())
	}
	return resp, bodyResponse, nil
}

func (b *RequestBuilder) UploadMultipart(method Method, url string, data *Multipart, headers ...interface{}) (*Response, error) {
	headers = append(headers, "Content-Type", data.ContentType)
	return b.SendRequest(method, url, data.Body, headers...)
}

func (b *RequestBuilder) UploadMultipartToStruct(method Method, url string, data *Multipart, to any, headers ...interface{}) ([]byte, error) {
	headers = append(headers, "Content-Type", data.ContentType)
	return b.SendRequestToStruct(method, url, data.Body, to, headers...)
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
