package gelbooru

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
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
	db.Image
	url string
}

type decoder struct {
	Sample          bool
	Directory, Hash string
	FileURL         string `json:"file_url"`
	Tags            string
}

// Ensure original tag is present and reuse map for deduplication
func (d *decoder) toImage(tag string, tmp map[string]struct{},
) (img image, err error) {
	// Dedup tags just in case. Boorus can't be trusted too much.
	split := strings.Split(d.Tags, " ")
	for k := range tmp {
		delete(tmp, k)
	}
	for _, t := range split {
		tmp[t] = struct{}{}
	}
	tmp[tag] = struct{}{} // Ensure map contains initial tag

	img.Tags = make([]string, 0, len(tmp))
	for k := range tmp {
		img.Tags = append(img.Tags, k)
	}

	img.MD5, err = common.DecodeMD5(d.Hash)
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

// Returns, if image is a valid target for fetching. Avoids downloading WebMs
// and GIFs.
func (d *decoder) isValid() bool {
	if d.Sample {
		return true
	}
	for _, s := range [...]string{"jpg", "jpeg", "png"} {
		if strings.HasSuffix(d.FileURL, s) {
			return true
		}
	}
	return false
}

// Fetch random matching file from Gelbooru.
// f can be nil, if no file is matched, even when err = nil.
// Caller must close and remove temporary file after use.
func Fetch(req common.FetchRequest) (f *os.File, image db.Image, err error) {
	mu.Lock()
	defer mu.Unlock()

	var w bytes.Buffer
	w.WriteString(
		"solo -photo -monochrome -multiple_girls -couple -multiple_boys -cosplay -objectification ")
	if !req.AllowExplicit {
		w.WriteString("rating:safe ")
	}
	w.WriteString(req.Tag)
	tags := w.String()

	pages, err := pageCount(req.Tag, tags)
	if err != nil || pages == 0 {
		return
	}
	images, err := fetchPage(req.Tag, tags, common.RandomInt(pages))
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

func pageCount(requested, tags string) (count int, err error) {
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
		images, err = fetchPage(requested, tags, page)
		if err != nil {
			return
		}
		exists = len(images) != 0
		if page == 0 && !exists { // These tags have only empty pages
			storePageCount(tags, 0)
			return
		}
		page += 5

		// Need a limit or we will hit the gelbooru page limit
		if page >= 200 {
			break
		}
	}
	for !exists { // Then find last non-empty page
		page--
		images, err = fetchPage(requested, tags, page)
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

func fetchPage(requested, tags string, page int) (images []image, err error) {
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
	tmp := make(map[string]struct{}, 128)
	for i, d := range dec {
		if !d.isValid() {
			continue
		}
		images[i], err = d.toImage(requested, tmp)
		if err != nil {
			return
		}
	}
	cache[tags].pages[page] = images
	return
}
