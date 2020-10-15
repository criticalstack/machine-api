package cloudinit

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/textproto"
)

const multipartHeader = `MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="%s"

`

func CreateMessage(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	b.WriteString(fmt.Sprintf(multipartHeader, w.Boundary()))
	part, err := w.CreatePart(textproto.MIMEHeader{
		"content-type": {"text/cloud-config"},
	})
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
