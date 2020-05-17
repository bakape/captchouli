package danbooru

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/bakape/boorufetch"
	"github.com/bakape/captchouli/v2/common"
	"github.com/bakape/captchouli/v2/db"
)

var (
	cache = make(map[string]*cacheEntry)
	mu    sync.Mutex

	blacklisted = map[string]struct{}{
		"photo":           {},
		"monochrome":      {},
		"multiple_girls":  {},
		"couple":          {},
		"multiple_boys":   {},
		"cosplay":         {},
		"objectification": {},
	}

	errAllFetched = errors.New("all pages fetched")
)

type cacheEntry struct {
	pages    map[int]struct{}
	maxPages int // Estimate for maximum number of pages
}

// Fetch random matching file from Danbooru.
// f can be nil, if no file is matched, even when err = nil.
// Caller must close and remove temporary file after use.
func Fetch(req common.FetchRequest) (f *os.File, image db.Image, err error) {
	mu.Lock()
	defer mu.Unlock()

	// Faster tag init
	skipPageFetch := false
	allFetched := false
	if req.IsInitial {
		var n int
		n, err = db.CountPending(req.Tag)
		if err != nil {
			return
		}
		skipPageFetch = n >= 3
	}
	if !skipPageFetch {
		err = tryFetchPage(req.Tag, req.Tag+" solo")
		switch err {
		case nil:
		case errAllFetched:
			err = nil
			allFetched = true
		default:
			return
		}
	}

	img, err := db.PopRandomPendingImage(req.Tag)
	if err != nil {
		if err == sql.ErrNoRows {
			if allFetched {
				err = common.ErrNoMatch
				return
			}
			err = nil
		}
		return
	}

	image = db.Image{
		Rating: img.Rating,
		Source: common.Danbooru,
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

// Attempt to fetch a random page from Danbooru
func tryFetchPage(requested, tags string) (err error) {
	store := cache[tags]
	if store == nil {
		maxPages := 300
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
		return errAllFetched
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

	posts, err := boorufetch.FromDanbooru(tags, uint(page), 100)
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

	// Push applicable posts to pending image set
	dst := make(chan error, 8)
	src := make(chan boorufetch.Post, len(posts))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, p := range posts {
		src <- p
	}
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case p := <-src:
					select {
					case <-ctx.Done():
						return
					case dst <- processPost(requested, p):
					}
				}

			}
		}()
	}
	for i := 0; i < len(posts); i++ {
		err = <-dst
		if err != nil {
			return
		}
	}

	// Set page as seen
	store.pages[page] = struct{}{}

	return
}

func processPost(requested string, p boorufetch.Post,
) (err error) {
	img := db.PendingImage{TargetTag: requested}
	img.MD5, err = p.MD5()
	if err != nil {
		// There are sometimes posts with no MD5 hash - ignore them
		return nil
	}

	// Check, if not already in DB
	inDB, err := db.IsInDatabase(img.MD5)
	if err != nil || inDB {
		return
	}
	inDB, err = db.IsPendingImage(img.MD5)
	if err != nil || inDB {
		return
	}

	blacklist := func() error {
		return db.BlacklistImage(img.MD5)
	}

	// File must be a still image
	valid := false
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
		return blacklist()
	}

	// Rating and tag fetches might need a network fetch, so do these later
	img.Rating, err = p.Rating()
	if err != nil {
		return
	}
	booruTags, err := p.Tags()
	if err != nil {
		return
	}

	hasChar := false
	hasSolo := false
	containsRequested := false
	for _, t := range booruTags {
		// Allow only images with 1 character in them
		if t.Type == boorufetch.Character {
			if hasChar {
				return blacklist()
			}
			hasChar = true
		}

		// Ensure tags do not contain any of the blacklisted tags
		if _, ok := blacklisted[t.Tag]; ok {
			return blacklist()
		}

		// Ensure tags contain solo
		if !hasSolo {
			hasSolo = t.Tag == "solo"
		}

		// Ensure array contains initial tag
		if !containsRequested {
			containsRequested = strings.ToLower(t.Tag) == requested
		}
	}
	if !containsRequested || !hasSolo {
		return blacklist()
	}

	img.Tags = make([]string, 0, len(booruTags))
	for _, t := range booruTags {
		img.Tags = append(img.Tags, t.Tag)
	}

	return db.InsertPendingImage(img)
}
