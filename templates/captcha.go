//go:generate qtc

package templates

import (
	"encoding/hex"
	"io"
	"os"

	"github.com/bakape/captchouli/common"
	"github.com/valyala/quicktemplate"
)

func streamencodeID(w *quicktemplate.Writer, id [64]byte) {
	hex.NewEncoder(w.W()).Write(id[:])
}

func streamthumbnail(w *quicktemplate.Writer, id [16]byte, tempBuf []byte) {
	f, err := os.Open(common.ThumbPath(id))
	if err != nil {
		return
	}
	defer f.Close()
	io.CopyBuffer(w.W(), f, tempBuf)
}
