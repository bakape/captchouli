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
	"net/url"
	"strconv"
	"strings"

	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
	"github.com/bakape/captchouli/templates"
	"github.com/dimfeld/httptreemux"
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

	// Optional function to execute on client failure to solve captcha
	OnFailure func(id [64]byte, r *http.Request) error
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
	err = s.initPool()
	if err != nil {
		return
	}
	if !s.opts.Quiet {
		log.Println("captchouli: service started")
	}
	return
}

// Initialize pool with enough images, if lacking
func (s *Service) initPool() (err error) {
	for _, t := range s.opts.Tags {
		err = s.initTag(t)
		if err != nil {
			return
		}
	}
	return
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
			if !s.opts.Quiet {
				log.Printf("captchouli: initializing image pool for tag `%s`\n",
					tag)
			}
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
func (s *Service) NewCaptcha(colour, background string, w io.Writer,
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
func (s *Service) CheckCaptcha(id [64]byte, solution []byte) error {
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
func (s *Service) Router() *httptreemux.ContextMux {
	r := httptreemux.NewContextMux()
	r.GET("/", s.ServeNewCaptcha)
	r.POST("/", s.ServeCheckCaptcha)
	r.POST("/status", s.ServeStatus)
	return r
}

// Generate new captcha and serve its HTML form
func (s *Service) ServeNewCaptcha(w http.ResponseWriter, r *http.Request) {
	handleError(w, s.ServeNewCaptchaErr(w, r))
}

// Like ServeNewCaptcha but returns error for custom error handling
func (s *Service) ServeNewCaptchaErr(w http.ResponseWriter, r *http.Request,
) (err error) {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	h := w.Header()
	for k, v := range headers {
		h.Set(k, v)
	}

	q := r.URL.Query()
	_, err = s.NewCaptcha(q.Get("captchouli-color"),
		q.Get("captchouli-background"), gw)
	return
}

// Serve POST requests for captcha solution validation
func (s *Service) ServeCheckCaptcha(w http.ResponseWriter, r *http.Request) {
	handleError(w, s.ServeCheckCaptchaError(w, r))
}

// Like ServeCheckCaptcha but returns error for custom error handling
func (s *Service) ServeCheckCaptchaError(w http.ResponseWriter, r *http.Request,
) (err error) {
	var (
		id       [64]byte
		solution []byte
	)
	id, solution, err = ExtractSolution(r)
	if err != nil {
		return
	}

	err = s.CheckCaptcha(id, solution)
	f := r.Form
	switch err {
	case nil:
		w.Write([]byte(f.Get("captchouli-id")))
	case ErrInvalidSolution:
		err = nil
		var (
			q url.Values
			u url.URL
		)
		for _, k := range [...]string{
			"captchouli-color", "captchouli-background",
		} {
			s := f.Get(k)
			if s != "" {
				if q == nil {
					q = make(url.Values)
				}
				q.Set(k, s)
			}
		}
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), 302)
		if s.opts.OnFailure != nil {
			err = s.opts.OnFailure(id, r)
		}
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

// Serve captcha solved status
func (s *Service) ServeStatus(w http.ResponseWriter, r *http.Request) {
	handleError(w, s.ServeStatusError(w, r))
}

// Like ServeStatus but returns error for custom error handling
func (s *Service) ServeStatusError(w http.ResponseWriter, r *http.Request,
) (err error) {
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

// Extact captcha ID and solution from request
func ExtractSolution(r *http.Request,
) (id [64]byte, solution []byte, err error) {
	id, err = ExtractID(r)
	if err != nil {
		return
	}

	solution = make([]byte, 0, 4)
	for i := 0; i < 9; i++ {
		s := r.Form.Get(solutionIDs[i])
		if s == "on" {
			solution = append(solution, byte(i))
		}
	}

	return
}

// Decode captcha ID from POSt request
func ExtractID(r *http.Request) (id [64]byte, err error) {
	err = r.ParseForm()
	if err != nil {
		return
	}
	return DecodeID(r.Form.Get("captchouli-id"))
}

// Decode captcha ID from base64 string
func DecodeID(s string) (id [64]byte, err error) {
	buf, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return
	}
	if len(buf) != 64 {
		err = ErrInvalidID
		return
	}
	copy(id[:], buf)
	return
}
