package captchouli

import (
	"bufio"
	"encoding/base64"
	"os"

	"github.com/bakape/captchouli/common"
)

func writeThumbnail(thumb []byte, md5 [16]byte) (err error) {
	f, err := os.OpenFile(common.ThumbPath(md5),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	bio := bufio.NewWriter(f)
	_, err = bio.WriteString(`data:image/jpeg;base64,`)
	if err != nil {
		return
	}
	enc := base64.NewEncoder(base64.StdEncoding, bio)
	_, err = enc.Write(thumb)
	if err != nil {
		return
	}
	err = enc.Close()
	if err != nil {
		return
	}
	return bio.Flush()
}
