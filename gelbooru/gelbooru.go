package gelbooru

import (
	"errors"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/bakape/boorufetch"
	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
)

var (
	cache        = make(map[string]*cacheEntry)
	mu           sync.Mutex
	errPageEmpty = errors.New("page empty")
)

type cacheEntry struct {
	pages    map[int]resultPage
	maxPages int // Estimate for maximum number of pages
}

type image struct {
	db.Image
	url string
}

// Simple stack for posts
type postStack struct {
	posts []boorufetch.Post
}

func (s *postStack) Push(p boorufetch.Post) {
	s.posts = append(s.posts, p)
}

func (s *postStack) Pop() boorufetch.Post {
	if len(s.posts) == 0 {
		return nil
	}

	i := len(s.posts) - 1
	p := s.posts[i]
	s.posts[i] = nil // Don't keep reference in backing array to enable GC
	s.posts = s.posts[:i]
	return p
}

type resultPage struct {
	requestedTag string
	posts        postStack
}

// Pop random image from page
func (p *resultPage) getImage() (img image, err error) {
	img.Source = common.Gelbooru
	var (
		dedup                map[string]struct{}
		tags                 []boorufetch.Tag
		hasChar, valid, inDB bool
	)
	for post := p.posts.Pop(); post != nil; post = p.posts.Pop() {
		img.MD5, err = post.MD5()
		if err != nil {
			return
		}
		inDB, err = db.IsInDatabase(img.MD5)
		if err != nil {
			return
		}
		if inDB {
			goto skip
		}

		// File must be a still image
		valid = false
		img.url = post.FileURL()
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

		// Rating and tag fetches might need a network fetch, so do these later

		img.Rating, err = post.Rating()
		if err != nil {
			return
		}

		if dedup == nil {
			// Only allocate when we actually need it
			dedup = make(map[string]struct{}, 128)
		} else {
			for k := range dedup {
				delete(dedup, k)
			}
		}

		hasChar = false
		tags, err = post.Tags()
		if err != nil {
			return
		}
		for _, t := range tags {
			// Allow only images with 1 character in them
			if t.Type == boorufetch.Character {
				if hasChar {
					err = db.BlacklistImage(img.MD5)
					if err != nil {
						return
					}
					goto skip
				}
				hasChar = true
			}
			// Dedup tags just in case. Boorus can't be trusted too much.
			dedup[t.Tag] = struct{}{}
		}
		dedup[p.requestedTag] = struct{}{} // Ensure map contains initial tag

		img.Tags = make([]string, 0, len(dedup))
		for t := range dedup {
			img.Tags = append(img.Tags, t)
		}

		return

	skip:
	}

	err = errPageEmpty
	return
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

	page, err := fetchPage(req.Tag, tags)
	if err != nil {
		return
	}

	img, err := page.getImage()
	switch err {
	case nil:
	case errPageEmpty:
		// Just skip this fetch
		err = nil
		return
	default:
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
func fetchPage(requested, tags string) (res resultPage, err error) {
	store := cache[tags]
	if store == nil {
		maxPages := 200
		if common.IsTest { // Reduce test duration
			maxPages = 2
		}
		store = &cacheEntry{
			pages:    make(map[int]resultPage),
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
	if len(store.pages) != 0 {
		page = common.RandomInt(store.maxPages)
	} else {
		page = 0
	}

	res, ok := store.pages[page]
	if ok { // Cache hit
		return
	}

	posts, err := boorufetch.FromGelbooru(tags, uint(page), 100)
	if err != nil {
		return
	}
	if len(posts) == 0 {
		if page == 0 {
			err = common.ErrNoMatch
			store.maxPages = 0 // Mark as invalid
			return
		}
		// Empty page. Don't check pages past this one. They will also be empty.
		store.maxPages = page
		// Retry with a new random page
		return fetchPage(requested, tags)
	}

	// Shuffle posts and push them to the stack
	res.requestedTag = requested
	rand.New(common.CryptoSource).Shuffle(len(posts), func(i, j int) {
		posts[i], posts[j] = posts[j], posts[i]
	})
	for _, p := range posts {
		res.posts.Push(p)
	}

	cache[tags].pages[page] = res
	return
}
