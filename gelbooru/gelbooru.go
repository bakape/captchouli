package gelbooru

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/bakape/boorufetch"
	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
)

var (
	cache = make(map[string]*cacheEntry)
	mu    sync.Mutex
)

type cacheEntry struct {
	pages    map[int][]image
	maxPages int // Estimate for maximum number of pages
}

type image struct {
	db.Image
	url string
}

// Fetch random matching file from Gelbooru.
// f can be nil, if no file is matched, even when err = nil.
// Caller must close and remove temporary file after use.
func Fetch(req common.FetchRequest) (f *os.File, image db.Image, err error) {
	mu.Lock()
	defer mu.Unlock()

	tags :=
		"solo -photo -monochrome -multiple_girls -couple -multiple_boys -cosplay -objectification " +
			req.Tag

	images, err := fetchPage(req.Tag, tags)
	if err != nil || len(images) == 0 {
		return
	}

	img := images[common.RandomInt(len(images))]
	exists, err := db.IsInDatabase(img.MD5)
	if err != nil || exists {
		return
	}
	image = img.Image

	r, err := http.Get(img.url)
	if err != nil {
		return
	}
	defer r.Body.Close()

	f, err = ioutil.TempFile("", "")
	if err != nil {
		return
	}
	_, err = io.Copy(f, r.Body)
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		f = nil
	}
	return
}

// Fetch a random page from gelbooru or cache
func fetchPage(requested, tags string) (images []image, err error) {
	store := cache[tags]
	if store == nil {
		maxPages := 200
		if common.IsTest { // Reduce test duration
			maxPages = 2
		}
		store = &cacheEntry{
			pages:    make(map[int][]image),
			maxPages: maxPages,
		}
		cache[tags] = store
	}
	if store.maxPages == 0 {
		err = common.ErrNoMatch
		return
	}

	// Always dowload first page on fresh fetch
	var page int
	if len(store.pages) == 0 {
		page = common.RandomInt(store.maxPages)
	} else {
		page = 1
	}

	images, ok := store.pages[page]
	if ok { // Cache hit
		return
	}

	limit := uint(100)
	if common.IsTest { // Reduce test duration
		limit = 5
	}
	posts, err := boorufetch.FromGelbooru(tags, uint(page), limit)
	switch {
	case err == nil:
	case err == io.EOF || len(posts) == 0:
		if page == 1 {
			err = common.ErrNoMatch
			store.maxPages = 0 // Mark as invalid
			return
		}
		// Empty page. Don't check pages past this one. They will also be empty.
		store.maxPages = page
		// Retry with a new random page
		return fetchPage(requested, tags)
	default:
		return
	}

	images = make([]image, 0, len(posts))
	var (
		valid bool
		img   = image{
			Image: db.Image{
				Source: common.Gelbooru,
			},
		}
		dedup = make(map[string]struct{}, 128)
	)
	for _, p := range posts {
		for k := range dedup {
			delete(dedup, k)
		}
		hasChar := false
		for _, t := range p.Tags() {
			// Allow only images with 1 character in them
			if t.Type == boorufetch.Character {
				if hasChar {
					goto skip
				}
				hasChar = true
			}
			// Dedup tags just in case. Boorus can't be trusted too much.
			dedup[t.Tag] = struct{}{}
		}
		dedup[requested] = struct{}{} // Ensure map contains initial tag

		// File must be a still image
		valid = false
		img.url = p.FileURL()
		if img.url == "" {
			goto skip
		}
		for _, s := range [...]string{"jpg", "jpeg", "png"} {
			if strings.HasSuffix(img.url, s) {
				valid = true
				break
			}
		}
		if !valid {
			goto skip
		}

		img.MD5, err = p.MD5()
		if err != nil {
			return
		}
		img.Rating = p.Rating()
		img.Tags = make([]string, 0, len(dedup))
		for t := range dedup {
			img.Tags = append(img.Tags, t)
		}

		images = append(images, img)
	skip:
	}

	cache[tags].pages[page] = images
	return
}
