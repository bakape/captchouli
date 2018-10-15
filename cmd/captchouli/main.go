package main

import (
	"log"
	"net/http"

	"github.com/bakape/captchouli"
)

var defaultTags = [...]string{
	"patchouli_knowledge", "cirno", "hakurei_reimu", "kirisame_marisa",
	"ibuki_suika", "konpaku_youmu", "smug",
}

func main() {
	var s *captchouli.Service
	err := func() (err error) {
		err = captchouli.Open()
		if err != nil {
			return
		}

		s, err = captchouli.NewService(captchouli.Options{
			Tags: defaultTags[:],
		})
		return
	}()
	if err != nil {
		panic(err)
	}
	defer captchouli.Close()

	log.Println(http.ListenAndServe(":8003", s))
}
