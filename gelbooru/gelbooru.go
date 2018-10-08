package gelbooru

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"

	"github.com/bakape/captchouli/common"
)

var (
	cache = make(map[string]cacheEntry)
	mu    sync.Mutex
)

type cacheEntry struct {
	pages     map[int][]image
	pageCount int
}

type image struct {
	hash [16]byte
	url  string
}

type decoder struct {
	Sample          bool
	Directory, Hash string
	FileURL         string `json:"file_url"`
}

func (d *decoder) toImage() (img image, err error) {
	img.hash, err = common.DecodeMD5(d.Hash)
	if err != nil {
		return
	}

	if d.Sample {
		img.url = fmt.Sprintf(
			"https://simg3.gelbooru.com/samples/%s/sample_%s.jpg",
			d.Directory, d.Hash)
	} else {
		img.url = d.FileURL
	}

	return
}

// Fetch random matching file from Gelbooru.
// f can be nil even, if no file is matched, even when err = nil.
// Caller must close and remove temporary file after use.
func Fetch(req common.FetchRequest) (f *os.File, hash [16]byte, err error) {
	mu.Lock()
	defer mu.Unlock()

	tags := fmt.Sprintf("solo rating:%s %s", req.Rating, req.Tag)

	pages, err := pageCount(tags)
	if err != nil || pages == 0 {
		return
	}
	images, err := fetchPage(tags, common.RandomInt(pages))
	if err != nil || len(images) == 0 {
		return
	}

	img := images[common.RandomInt(len(images))]
	hash = img.hash
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

func pageCount(tags string) (count int, err error) {
	entry, ok := cache[tags]
	if ok {
		count = entry.pageCount
		return
	}
	entry.pages = make(map[int][]image, 16)
	cache[tags] = entry

	var (
		page   = 0
		exists = true
		images []image
	)
	for exists { // First find first empty page in increments of 5
		images, err = fetchPage(tags, page)
		if err != nil {
			return
		}
		exists = len(images) != 0
		if page == 0 && !exists { // These tags have only empty pages
			storePageCount(tags, 0)
			return
		}
		page += 5
	}
	for !exists { // Then find last non-empty page
		page--
		images, err = fetchPage(tags, page)
		if err != nil {
			return
		}
		exists = len(images) != 0
	}

	count = page + 1
	storePageCount(tags, count)
	return
}

func storePageCount(tags string, count int) {
	entry := cache[tags]
	entry.pageCount = count
	cache[tags] = entry
}

func fetchPage(tags string, page int) (images []image, err error) {
	images, ok := cache[tags].pages[page]
	if ok {
		return
	}

	u := url.URL{
		Scheme: "https",
		Host:   "gelbooru.com",
		Path:   "/index.php",
		RawQuery: url.Values{
			"page":  {"dapi"},
			"s":     {"post"},
			"q":     {"index"},
			"json":  {"1"},
			"tags":  {tags},
			"limit": {"100"},
			"pid":   {strconv.Itoa(page)},
		}.Encode(),
	}
	r, err := http.Get(u.String())
	if err != nil {
		return
	}
	defer r.Body.Close()

	var dec []decoder
	err = json.NewDecoder(r.Body).Decode(&dec)
	if err != nil || len(dec) == 0 {
		return
	}
	images = make([]image, len(dec))
	for i := range dec {
		images[i], err = dec[i].toImage()
		if err != nil {
			return
		}
	}
	cache[tags].pages[page] = images
	return
}
