package captchouli

import (
	"encoding/hex"
	"io/ioutil"
	"path/filepath"

	"github.com/bakape/captchouli/common"
)

func thumbPath(md5 [16]byte) string {
	return filepath.Join(common.RootDir, "images", hex.EncodeToString(md5[:]))
}

func writeThumbnail(thumb []byte, md5 [16]byte) error {
	return ioutil.WriteFile(thumbPath(md5), thumb, 0600)
}
