package gelbooru

import (
	"database/sql"
	"io"
	"io/ioutil"
	"log"
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
	pages    map[int]struct{}
	maxPages int // Estimate for maximum number of pages
}

// Fetch random matching file from Gelbooru.
// f can be nil, if no file is matched, even when err = nil.
// Caller must close and remove temporary file after use.
func Fetch(req common.FetchRequest) (f *os.File, image db.Image, err error) {
	mu.Lock()
	defer mu.Unlock()

	tags := "solo -photo -monochrome -multiple_girls -couple -multiple_boys " +
		"-cosplay -objectification " +
		req.Tag

	err = tryFetchPage(req.Tag, tags)
	if err != nil {
		return
	}

	img, err := db.PopRandomPendingImage(req.Tag)
	if err != nil {
		if err == sql.ErrNoRows {
			err = nil
		}
		return
	}

	image = db.Image{
		Rating: img.Rating,
		Source: req.Source,
		MD5:    img.MD5,
		Tags:   img.Tags,
	}

	r, err := http.Get(img.URL)
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
		// Ignore any errors here. This cleanup need not succeed.
		f.Close()
		os.Remove(f.Name())
		f = nil
	}
	return
}

// Attempt to fetch a random page from gelbooru
func tryFetchPage(requested, tags string) (err error) {
	store := cache[tags]
	if store == nil {
		maxPages := 200
		if common.IsTest { // Reduce test duration
			maxPages = 10
		}
		store = &cacheEntry{
			pages:    make(map[int]struct{}),
			maxPages: maxPages,
		}
		cache[tags] = store
	}
	if store.maxPages == 0 {
		err = common.ErrNoMatch
		return
	}
	if len(store.pages) == store.maxPages {
		// Already fetched all pages
		return
	}

	// Always dowload first page on fresh fetch
	var page int
	if len(store.pages) != 0 {
		page = common.RandomInt(store.maxPages)
	} else {
		page = 0
	}

	_, ok := store.pages[page]
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
		return tryFetchPage(requested, tags)
	}

	// Push applicable posts to pending image set.
	// Reuse allocated resources, where possible.
	var (
		booruTags                      []boorufetch.Tag
		img                            = db.PendingImage{TargetTag: requested}
		hasChar, valid, inDB, contains bool
	)
	for i, p := range posts {
		if common.IsTest && i >= 10 {
			break // Shorten tests
		}
		img.MD5, err = p.MD5()
		if err != nil {
			return
		}

		// Check, if not already in DB
		//
		// TODO: Perform this in one query on array of bytea hashes
		inDB, err = db.IsInDatabase(img.MD5)
		if err != nil {
			return
		}
		if inDB {
			continue
		}
		inDB, err = db.IsPendingImage(img.MD5)
		if err != nil {
			return
		}
		if inDB {
			continue
		}

		// File must be a still image
		valid = false
		img.URL = p.FileURL()
		if img.URL != "" {
			for _, s := range [...]string{"jpg", "jpeg", "png"} {
				if strings.HasSuffix(img.URL, s) {
					valid = true
					break
				}
			}
		}
		if !valid {
			err = db.BlacklistImage(img.MD5)
			if err != nil {
				return
			}
			continue
		}

		// Rating and tag fetches might need a network fetch, so do these later

		img.Rating, err = p.Rating()
		if err != nil {
			return
		}

		hasChar = false
		booruTags, err = p.Tags()
		if err != nil {
			return
		}
		for _, t := range booruTags {
			// Allow only images with 1 character in them and ensure said
			// character matches the requested tag in case of gelbooru-danbooru
			// desync
			if t.Type == boorufetch.Character {
				if hasChar ||
					// Ensure no case mismatch, as tags are queried as lowercase
					// in the boorus
					strings.ToLower(t.Tag) != strings.ToLower(requested) {
					err = db.BlacklistImage(img.MD5)
					if err != nil {
						return
					}
					goto skip
				}
				hasChar = true
			}
		}

		contains = false
		for _, t := range booruTags {
			if t.Tag == requested {
				contains = true
				break
			}
		}
		img.Tags = make([]string, 0, len(booruTags))
		for _, t := range booruTags {
			img.Tags = append(img.Tags, t.Tag)
		}
		if !contains {
			// Ensure array contains initial tag
			img.Tags = append(img.Tags, requested)
		}

		err = db.InsertPendingImage(img)
		if err != nil {
			return
		}
		if common.IsTest {
			log.Printf("logged pending image: %s\n", img.URL)
		}

	skip:
	}

	// Set page as seen
	store.pages[page] = struct{}{}

	return
}
