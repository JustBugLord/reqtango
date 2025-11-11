package reqtango

import "bytes"

type Multipart struct {
	ContentType string
	Body        *bytes.Buffer
}
