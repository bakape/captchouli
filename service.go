package captchouli

import (
	"compress/gzip"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
	"github.com/bakape/captchouli/templates"
)

var headers = map[string]string{
	"Cache-Control":               "no-store",
	"Access-Control-Allow-Origin": "*",
	"Content-Encoding":            "gzip",
	"Content-Type":                "text/html",
}

// Allow images with explicit content. Note that this only applies to
// fetching new images for the pool. Once your pool has any explicit images,
// they will be selected for captchas like any other image.
var AllowExplicit = false

// Source of image database to use for captcha image generation
type DataSource = common.DataSource

const Gelbooru = common.Gelbooru

const (
	// minimum size of image pool for a tag
	poolMinSize = 6
)

// Options passed on Service creation
type Options struct {
	// Source of image database to use for captcha image generation
	Source DataSource

	// Tags to source for captcha solutions. One tag is randomly chosen for each
	// generated captcha. Required to contain at least 1 tag but the database
	// must include at least 3 different tags for correct operation.
	//
	// Note that you can only include tags that are discernable from the
	// character's face, such as who the character is (example: "cirno") or a
	// facial feature of the character (example: "smug").
	Tags []string
}

// Encapsulates a configured captcha-generation and verification service
type Service struct {
	allowExplicit bool
	source        DataSource
	urlOverride   string
	tags          []string
}

// Create new captcha-generation and verification service
func NewService(opts Options) (s *Service, err error) {
	if len(opts.Tags) == 0 {
		err = Error{errors.New("no tags provided")}
		return
	}

	s = &Service{
		source: opts.Source,
		tags:   opts.Tags,
	}

	err = initClassifier(opts.Source)
	if err != nil {
		return
	}
	err = s.initPool()
	if err != nil {
		return
	}
	log.Println("captchouli: service started")
	return
}

// Initialize pool with enough images, if lacking
func (s *Service) initPool() (err error) {
	for _, t := range s.tags {
		err = initPool(t, s.source)
		if err != nil {
			return
		}
	}
	return
}

func initPool(tag string, source common.DataSource) (err error) {
	var (
		count int
		first = true
	)
	for {
		count, err = db.ImageCount(tag, source)
		if err != nil {
			return
		}
		if count >= poolMinSize {
			return
		} else if first {
			first = false
			log.Printf("captchouli: initializing image pool for tag `%s`\n",
				tag)
		}

		err = fetch(common.FetchRequest{
			Tag:    tag,
			Source: source,
		})
		if err != nil {
			return
		}
	}
}

// Creates a new captcha, writes its HTML contents to w and returns captcha ID.
//
// Depending on what type w is, you might want to buffer the output with
// bufio.NewWriter.
func (s *Service) NewCaptcha(colour, background string, w io.Writer,
) (id [64]byte, err error) {
	tag := s.tags[common.RandomInt(len(s.tags))]
	id, images, err := db.GenerateCaptcha(tag, s.source)
	if err != nil {
		return
	}

	if background == "" {
		background = "#d6daf0"
	}
	if colour == "" {
		colour = "black"
	}

	templates.WriteCaptcha(w, colour, background, tag, id, images)

	if !common.IsTest {
		scheduleFetch <- common.FetchRequest{AllowExplicit, tag, s.source}
	}
	return
}

// Serves new captcha generation request with GZIP compression
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := s.ServeHTTPError(w, r)
	if err != nil {
		http.Error(w, err.Error(), 500)
		log.Println(err)
	}
}

// Like ServeHTTP() but passes any error to caller. Allows for custom error
// handling.
func (s *Service) ServeHTTPError(w http.ResponseWriter, r *http.Request,
) (err error) {
	h := w.Header()
	for k, v := range headers {
		h.Set(k, v)
	}

	gw := gzip.NewWriter(w)
	q := r.URL.Query()
	_, err = s.NewCaptcha(q.Get("color"), q.Get("background"), gw)
	if err != nil {
		return
	}
	return gw.Close()
}
