package captchouli

import (
	"log"
	"os"
	"time"

	"github.com/bakape/captchouli/v2/common"
	"github.com/bakape/captchouli/v2/danbooru"
	"github.com/bakape/captchouli/v2/db"
)

var (
	scheduleFetch = make(chan common.FetchRequest, 256)
)

func init() {
	go func() {
		requests := make(map[common.FetchRequest]struct{})
		tick := time.Tick(time.Second)

		for {
			select {
			case req := <-scheduleFetch:
				requests[req] = struct{}{} // Deduplicate request
			case <-tick:
				if len(requests) == 0 {
					break
				}

				// Get random request
				target := common.RandomInt(len(requests))
				i := 0
				for req := range requests {
					if i == target {
						err := fetch(req)
						if err != nil {
							log.Printf("fetch error on tag `%s`\n", req.Tag)
						}
						delete(requests, req)
						break
					}
					i++
				}
			}
		}
	}()
}

func fetch(req common.FetchRequest) (err error) {
	f, img, err := danbooru.Fetch(req)
	if f == nil || err != nil {
		return
	}
	defer os.Remove(f.Name())
	defer f.Close()

	thumb, err := thumbnail(f.Name())
	switch err {
	case nil:
	case ErrNoFace:
		return db.BlacklistImage(img.MD5)
	default:
		return
	}
	err = writeThumbnail(thumb, img.MD5)
	if err != nil {
		return
	}
	return db.InsertImage(img)
}
