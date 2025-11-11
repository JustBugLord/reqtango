package reqtango

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
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
	return b.SendRequest("GET", url, nil, headers...)
}

func (b *RequestBuilder) GetToStruct(url string, to any, headers ...interface{}) error {
	return b.SendRequestToStruct("GET", url, nil, to, headers...)
}

func (b *RequestBuilder) Post(url, body string, headers ...interface{}) (*Response, error) {
	return b.SendRequest("POST", url, bytes.NewBuffer([]byte(body)), headers...)
}

func (b *RequestBuilder) PostToStruct(url, body string, to any, headers ...interface{}) error {
	return b.SendRequestToStruct("POST", url, bytes.NewBuffer([]byte(body)), to, headers...)
}

func (b *RequestBuilder) SendRequest(method, url string, body io.Reader, headers ...interface{}) (*Response, error) {
	resp, bodyResp, err := b.SendRequestRaw(method, url, body, headers...)
	if err != nil {
		return nil, err
	}
	return &Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Body:       string(bodyResp),
	}, nil
}

func (b *RequestBuilder) UploadFile(url, filePath string, headers ...interface{}) (*Response, error) {
	body, err := b.MultipartFromFile(filePath)
	if err != nil {
		return nil, err
	}
	headers = append(headers, "Content-Type", body.ContentType)
	return b.SendRequest("POST", url, body.Body, headers...)
}

func (b *RequestBuilder) UploadFileToStruct(url, filePath string, to any, headers ...interface{}) error {
	body, err := b.MultipartFromFile(filePath)
	if err != nil {
		return err
	}
	headers = append(headers, "Content-Type", body.ContentType)
	return b.SendRequestToStruct("POST", url, body.Body, to, headers...)
}

func (b *RequestBuilder) SendRequestToStruct(method, url string, body io.Reader, to any, headers ...interface{}) error {
	_, data, err := b.SendRequestRaw(method, url, body, headers...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, to); err != nil {
		return errors.New("fail unmarshal response: " + err.Error())
	}
	return nil
}

func (b *RequestBuilder) SendRequestRaw(method, url string, body io.Reader, headers ...interface{}) (*http.Response, []byte, error) {
	req, err := http.NewRequest(method, url, body)
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

func (b *RequestBuilder) MultipartFromFile(filePath string) (*Multipart, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("fail file open: %w", err)
	}
	defer file.Close()
	fileName := filepath.Base(filePath)
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", contentType) // или другой нужный тип
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileName))
	fileWriter, err := bodyWriter.CreatePart(h)
	if err != nil {
		return nil, fmt.Errorf("fail create part: %w", err)
	}
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		return nil, fmt.Errorf("fail copy file in writer: %w", err)
	}
	err = bodyWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("fail close multipart writer: %w", err)
	}
	return &Multipart{
		ContentType: bodyWriter.FormDataContentType(),
		Body:        bodyBuf,
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
