package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/bakape/captchouli"
)

var defaultTags = [...]string{
	"patchouli_knowledge", "hakurei_reimu", "cirno", "kirisame_marisa",
	"konpaku_youmu", ":>",
}

func main() {
	address := flag.String("a", ":8512", "address for server to listen on")
	explicit := flag.Bool("e", false,
		"allow explicit rating images in the pool")
	tags := flag.String("t", strings.Join(defaultTags[:], ","),
		`Comma-separated list of tags to use in the pool. At least 3 required.
Note that only tags that are detectable from the character's face should be used.
`)

	flag.Parse()

	var s *captchouli.Service
	err := func() (err error) {
		err = captchouli.Open()
		if err != nil {
			return
		}

		tags := strings.Split(*tags, ",")
		if len(tags) < 3 {
			return fmt.Errorf("not enough tags provided")
		}
		opts := captchouli.Options{
			Tags: tags,
		}
		if *explicit {
			opts.Explicitness = []captchouli.Rating{captchouli.Safe,
				captchouli.Questionable, captchouli.Explicit}
		}

		s, err = captchouli.NewService(opts)
		return
	}()
	if err != nil {
		panic(err)
	}
	defer captchouli.Close()

	log.Println("listening on " + *address)
	log.Println(http.ListenAndServe(*address, s.Router()))
}
