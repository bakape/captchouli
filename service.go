package captchouli

import (
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
	"github.com/bakape/captchouli/templates"
	"github.com/julienschmidt/httprouter"
)

var (
	headers = map[string]string{
		"Cache-Control":               "no-store, private",
		"Access-Control-Allow-Origin": "*",
		"Content-Encoding":            "gzip",
		"Content-Type":                "text/html",
	}

	// Signifies the client had solved the captcha incorrectly
	ErrInvalidSolution = Error{errors.New("invalid captcha solution")}

	// Captcha ID is of invalid format
	ErrInvalidID = Error{errors.New("invalid captcha id")}

	// Prebuilt and cached
	solutionIDs [9]string
)

func init() {
	for i := 0; i < 9; i++ {
		solutionIDs[i] = fmt.Sprintf("captchouli-%d", i)
	}
}

// Source of image database to use for captcha image generation
type DataSource = common.DataSource

const Gelbooru = common.Gelbooru

const (
	// minimum size of image pool for a tag
	poolMinSize = 6
)

// Options passed on Service creation
type Options struct {
	// Allow images with explicit content. Note that this only applies to
	// fetching new images for the pool. Once your pool has any explicit images,
	// they will be selected for captchas like any other image.
	AllowExplicit bool

	// Silence non-error log outputs
	Quiet bool

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
	opts Options
}

// Create new captcha-generation and verification service
func NewService(opts Options) (s *Service, err error) {
	if len(opts.Tags) == 0 {
		err = Error{errors.New("no tags provided")}
		return
	}

	s = &Service{opts}

	err = initClassifier(opts.Source)
	if err != nil {
		return
	}
	s.initPool()
	if !s.opts.Quiet {
		log.Println("captchouli: service started")
	}
	return
}

// Initialize pool with enough images, if lacking
func (s *Service) initPool() {
	completed := make([]string, 0, len(s.opts.Tags))
	for _, t := range s.opts.Tags {
		err := s.initTag(t)
		if err != nil {
			log.Printf(
				"captchouli: error initializing image pool for tag `%s`: %s",
				t, err)
		} else {
			completed = append(completed, t)
		}
	}
	s.opts.Tags = completed
}

func (s *Service) initTag(tag string) (err error) {
	var (
		count int
		first = true
	)
	for {
		count, err = db.ImageCount(tag, s.opts.Source)
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
			Source: s.opts.Source,
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
func (s *Service) NewCaptcha(w io.Writer, colour, background string,
) (id [64]byte, err error) {
	tag := s.opts.Tags[common.RandomInt(len(s.opts.Tags))]
	id, images, err := db.GenerateCaptcha(tag, s.opts.Source)
	if err != nil {
		return
	}

	if background == "" {
		background = "#d6daf0"
	}
	if colour == "" {
		colour = "black"
	}

	templates.WriteCaptcha(w, colour, background,
		html.EscapeString(strings.Title(strings.Replace(tag, "_", " ", -1))),
		id, images)

	if !common.IsTest {
		scheduleFetch <- common.FetchRequest{s.opts.AllowExplicit, tag,
			s.opts.Source}
	}
	return
}

// Check a captcha solution for validity.
// solution: slice of selected image numbers
func CheckCaptcha(id [64]byte, solution []byte) error {
	solved, err := db.CheckSolution(id, solution)
	if err != nil {
		return err
	} else if !solved {
		return ErrInvalidSolution
	}
	return nil
}

// Creates a routed handler for serving the API.
// The router implements http.Handler.
func (s *Service) Router() *httprouter.Router {
	r := httprouter.New()
	r.HandlerFunc("GET", "/", func(w http.ResponseWriter, r *http.Request) {
		handleError(w, s.ServeNewCaptcha(w, r))
	})
	r.HandlerFunc("POST", "/", func(w http.ResponseWriter,
		r *http.Request,
	) {
		handleError(w, s.ServeCheckCaptcha(w, r))
	})
	r.HandlerFunc("POST", "/status", func(w http.ResponseWriter,
		r *http.Request,
	) {
		handleError(w, ServeStatus(w, r))
	})
	return r
}

// Generate new captcha and serve its HTML form
func (s *Service) ServeNewCaptcha(w http.ResponseWriter, r *http.Request,
) (err error) {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	h := w.Header()
	for k, v := range headers {
		h.Set(k, v)
	}

	err = r.ParseForm()
	if err != nil {
		return
	}
	_, err = s.NewCaptcha(gw, r.Form.Get(ColourKey), r.Form.Get(BackgroundKey))
	return
}

// Serve POST requests for captcha solution validation
func (s *Service) ServeCheckCaptcha(w http.ResponseWriter, r *http.Request,
) (err error) {
	id, err := ExtractID(r)
	if err != nil {
		return
	}
	solution, err := ExtractSolution(r)
	if err != nil {
		return
	}

	err = CheckCaptcha(id, solution)
	switch err {
	case nil:
		dst := make([]byte, base64.StdEncoding.EncodedLen(len(id)))
		base64.StdEncoding.Encode(dst, id[:])
		w.Write(dst)
	case ErrInvalidSolution:
		err = s.ServeNewCaptcha(w, r)
	}
	return
}

func handleError(w http.ResponseWriter, err error) {
	code := 500
	switch err {
	case nil:
		return
	case ErrInvalidID:
		code = 400
	}
	http.Error(w, err.Error(), code)
}

// Serve captcha solved status. The captcha is deleted on a successful check to
// prevent replayagain attacks.
func ServeStatus(w http.ResponseWriter, r *http.Request) (err error) {
	id, err := ExtractID(r)
	if err != nil {
		return
	}
	solved, err := db.IsSolved(id)
	if err != nil {
		return
	}
	w.Write(strconv.AppendBool(nil, solved))
	return
}
