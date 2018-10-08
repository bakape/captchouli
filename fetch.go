package captchouli

import (
	"log"
	"os"
	"time"

	"github.com/bakape/captchouli/common"
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
						fetch(req)
						delete(requests, req)
						break
					}
					i++
				}
			}
		}
	}()
}

func fetch(req common.FetchRequest) {
	var fn func(common.FetchRequest) (*os.File, [16]byte, error)
	switch req.Source {
	case common.Gelbooru:
		fn = gelbooru.Fetch
	}
	f, _, err := fn(req)
	if err != nil {
		log.Printf("fetch error: from %s on tag %s", req.Source, req.Tag)
	}
	defer os.Remove(f.Name())
	defer f.Close()
}
