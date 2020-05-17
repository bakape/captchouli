package captchouli

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/bakape/boorufetch"
	"github.com/bakape/captchouli/v2/common"
	"github.com/bakape/captchouli/v2/db"
	"github.com/bakape/captchouli/v2/templates"
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

const (
	Gelbooru = common.Gelbooru
	Danbooru = common.Danbooru
)

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

	// Allow images with varying explicitness. Defaults to only Safe.
	Explicitness []Rating

	// Tags to source for captcha solutions. One tag is randomly chosen for each
	// generated captcha. Required to contain at least 3 tags.
	//
	// Note that you can only include tags that are discernable from the
	// character's face, such as who the character is (example: "cirno") or a
	// facial feature of the character (example: "smug").
	Tags []string
}

// Encapsulates a configured captcha-generation and verification service
type Service struct {
	quiet           bool
	explicitnessStr string
	explicitness    []Rating
	tags            appendSlice
}

// Slice with thread-safe appending
type appendSlice struct {
	mu    sync.RWMutex
	inner []string
}

func (s *appendSlice) Get() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inner
}

func (s *appendSlice) Append(extra string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inner = append(s.inner, extra)
}

// Create new captcha-generation and verification service
func NewService(opts Options) (s *Service, err error) {
	if len(opts.Tags) < 3 {
		err = Error{errors.New("at least 3 tags required")}
		return
	}

	s = &Service{
		quiet:        opts.Quiet,
		explicitness: opts.Explicitness,
	}
	if len(s.explicitness) == 0 {
		s.explicitness = []Rating{Safe}
	}

	err = initClassifier()
	if err != nil {
		return
	}
	err = s.initPool(opts.Tags)
	if err != nil {
		return
	}

	if !s.quiet {
		log.Println("captchouli: service started")
	}
	return
}

// Initialize pool with enough images, if lacking
func (s *Service) initPool(tags []string) (err error) {
	formatErr := func(tag string, err error) error {
		return Error{
			Err: fmt.Errorf(
				"error initializing image pool for tag `%s`: %w",
				tag,
				err,
			),
		}
	}

	// Init first 3 tags needed for operation first and init the rest
	// eventually to reduce startup times
	for _, tag := range tags[:3] {
		err = s.initTag(tag)
		if err != nil {
			return formatErr(tag, err)
		}
		s.tags.Append(tag)
	}
	if len(tags) > 3 {
		go func() {
			for _, tag := range tags[3:] {
				err = s.initTag(tag)
				if err != nil {
					log.Print(formatErr(tag, err))
				} else {
					s.tags.Append(tag)
				}
			}
		}()
	}
	return
}

func (s *Service) formatExplicitness() string {
	if s.explicitnessStr != "" {
		return s.explicitnessStr
	}

	var w bytes.Buffer
	w.WriteByte('[')
	for i, r := range s.explicitness {
		if i != 0 {
			w.WriteString(", ")
		}
		w.WriteString(r.String())
	}
	w.WriteByte(']')
	s.explicitnessStr = w.String()
	return s.explicitnessStr
}

func (s *Service) initTag(tag string) (err error) {
	var (
		count, fetchCount int
		first             = true
		f                 = s.filters(tag)
		req               = f.FetchRequest
	)
	req.IsInitial = true
	for {
		count, err = db.ImageCount(f)
		if err != nil {
			return
		}
		if count >= poolMinSize {
			// Terminate open line
			if fetchCount != 0 {
				fmt.Print("\n")
			}
			return
		} else if first {
			first = false
			log.Printf(
				"captchouli: initializing tag=%s explicitness=%s\n",
				tag,
				s.formatExplicitness(),
			)
		}

		if !s.quiet {
			fetchCount++
			fmt.Printf("captchouli: image fetch: %d\n", fetchCount)
		}
		err = fetch(req)
		if err != nil {
			return
		}
	}
}

func (s *Service) filters(tag string) db.Filters {
	return db.Filters{
		FetchRequest: common.FetchRequest{
			Tag: tag,
		},
		Explicitness: s.explicitness,
	}
}

// Creates a new captcha, writes its HTML contents to w and returns captcha ID.
//
// Depending on what type w is, you might want to buffer the output with
// bufio.NewWriter.
func (s *Service) NewCaptcha(w io.Writer, colour, background string,
) (id [64]byte, err error) {
	tags := s.tags.Get()
	tag := tags[common.RandomInt(len(tags))]
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
		FetchRequest: common.FetchRequest{
			Tag: tag,
		},
		Explicitness: s.explicitness,
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
