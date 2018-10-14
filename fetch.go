package captchouli

import (
	"log"
	"os"
	"time"

	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
	"github.com/bakape/captchouli/gelbooru"
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
							log.Printf("fetch error: from %s on tag `%s`\n",
								req.Source, req.Tag)
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
	var fn func(common.FetchRequest) (*os.File, db.Image, error)
	switch req.Source {
	case common.Gelbooru:
		fn = gelbooru.Fetch
	}
	f, img, err := fn(req)
	if f == nil {
		return
	}
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	defer f.Close()

	thumb, err := thumbnail(f.Name(), req.Source)
	switch err {
	case nil:
	case ErrNoFace:
		return nil
	default:
		return
	}
	err = writeThumbnail(thumb, img.MD5)
	if err != nil {
		return
	}
	return db.InsertImage(img)
}
