package captchouli

import (
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/bakape/boorufetch"
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

// Explicitness rating of image
type Rating = boorufetch.Rating

const (
	Safe Rating = iota
	Questionable
	Explicit
)

// Options passed on Service creation
type Options struct {
	// Silence non-error log outputs
	Quiet bool

	// Source of image database to use for captcha image generation
	Source DataSource

	// Allow images with varying explicitness. Defaults to only Safe.
	Explicitness []Rating

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
	if len(s.opts.Explicitness) == 0 {
		s.opts.Explicitness = []Rating{Safe}
	}

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
		f     = s.filters(tag)
		req   = f.FetchRequest
	)
	for {
		count, err = db.ImageCount(f)
		if err != nil {
			return
		}
		if count >= poolMinSize {
			return
		} else if first {
			first = false
			log.Printf("captchouli: initializing tag=%s explicitness=%v\n",
				tag, s.opts.Explicitness)
		}

		err = fetch(req)
		if err != nil {
			return
		}
	}
}

func (s *Service) request(tag string) common.FetchRequest {
	return common.FetchRequest{
		Tag:    tag,
		Source: s.opts.Source,
	}
}

func (s *Service) filters(tag string) db.Filters {
	return db.Filters{
		FetchRequest: s.request(tag),
		Explicitness: s.opts.Explicitness,
	}
}

// Creates a new captcha, writes its HTML contents to w and returns captcha ID.
//
// Depending on what type w is, you might want to buffer the output with
// bufio.NewWriter.
func (s *Service) NewCaptcha(w io.Writer, colour, background string,
) (id [64]byte, err error) {
	tag := s.opts.Tags[common.RandomInt(len(s.opts.Tags))]
	f := s.filters(tag)
	n, err := db.ImageCount(f)
	if err != nil {
		return
	}
	if n < 4 {
		// Not enough to generate captcha. Schedule a fetch and try a different
		// tag.
		if !common.IsTest {
			scheduleFetch <- f.FetchRequest
		}
		return s.NewCaptcha(w, colour, background)
	}

	id, images, err := db.GenerateCaptcha(db.Filters{
		FetchRequest: s.request(tag),
		Explicitness: s.opts.Explicitness,
	})
	if err != nil {
		return
	}

	if background == "" {
		background = "#d6daf0"
	}
	if colour == "" {
		colour = "black"
	}

	tagF := strings.Replace(tag, "_", " ", -1)
	if len(tagF) != 0 {
		// Don't title() tags of emoticons
		switch tagF[0] {
		case ';', ':', '=':
		default:
			tagF = strings.Title(tagF)
		}
	}
	templates.WriteCaptcha(w, colour, background, tagF, id, images)

	if !common.IsTest {
		scheduleFetch <- f.FetchRequest
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
