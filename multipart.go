package reqtango

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

type RecordType string

const (
	File RecordType = "file"
	Text RecordType = "text"
)

type Record struct {
	RecordType RecordType
	Value      string
}

type Multipart struct {
	ContentType string
	Body        *bytes.Buffer
}

type MultipartBuilder struct {
	records map[string]Record
}

func NewMultipartBuilder() *MultipartBuilder {
	return &MultipartBuilder{
		records: map[string]Record{},
	}
}

func (m *MultipartBuilder) AddFileByPath(fieldName, filePath string) {
	m.records[fieldName] = Record{
		RecordType: File,
		Value:      filePath,
	}
}

func (m *MultipartBuilder) AddField(fieldName, fieldValue string) {
	m.records[fieldName] = Record{
		RecordType: Text,
		Value:      fieldValue,
	}
}

func (m *MultipartBuilder) Build() (*Multipart, error) {
	if m.records == nil || len(m.records) == 0 {
		return nil, errors.New("empty records")
	}
	data := map[string]io.Reader{}
	for key, reader := range m.records {
		if reader.RecordType == File {
			file, err := os.Open(reader.Value)
			if err != nil {
				return nil, err
			}
			data[key] = file
		} else if reader.RecordType == Text {
			data[key] = strings.NewReader(reader.Value)
		}
	}
	return RawMultipart(data)
}

func RawMultipart(data map[string]io.Reader) (*Multipart, error) {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	for key, reader := range data {
		if file, ok := reader.(*os.File); ok {
			fileName := filepath.Base(file.Name())
			contentType := mime.TypeByExtension(filepath.Ext(fileName))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			h := make(textproto.MIMEHeader)
			h.Set("Content-Type", contentType)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileName))
			fileWriter, err := bodyWriter.CreatePart(h)
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(fileWriter, file)
			if err != nil {
				return nil, err
			}
		} else {
			var sb strings.Builder
			_, err := io.Copy(&sb, reader)
			if err != nil {
				return nil, err
			}
			err = bodyWriter.WriteField(key, sb.String())
			if err != nil {
				return nil, err
			}
		}
	}
	multipartResult := &Multipart{
		ContentType: bodyWriter.FormDataContentType(),
		Body:        bodyBuf,
	}
	err := bodyWriter.Close()
	if err != nil {
		return nil, err
	}
	return multipartResult, nil
}
