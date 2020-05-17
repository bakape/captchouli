package danbooru

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/bakape/captchouli/common"
	"github.com/bakape/captchouli/db"
	"github.com/olekukonko/tablewriter"
)

func TestMain(t *testing.M) {
	db.OpenForTests()
	os.Exit(t.Run())
}

func TestFetch(t *testing.T) {
	testFetches(t, "sakura_kyouko")
}

func testFetches(t *testing.T, tag string) {
	t.Helper()

	var buf bytes.Buffer
	w := tablewriter.NewWriter(&buf)
	w.SetAlignment(tablewriter.ALIGN_LEFT)
	w.SetColWidth(80)
	w.SetRowLine(true)
	w.SetHeader([]string{"rating", "MD5", "tags"})

	f, img, err := Fetch(common.FetchRequest{
		Tag: tag,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		err = os.Remove(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		err = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		w.Append([]string{img.Rating.String(),
			hex.EncodeToString(img.MD5[:]),
			fmt.Sprint(img.Tags)})
	}

	w.Render()
	t.Logf("\n%s\n", buf.String())
}

func TestNoMatch(t *testing.T) {
	_, _, err := Fetch(common.FetchRequest{
		Tag: "sakura_kyouko_dsadsdadsadsad",
	})
	if err != common.ErrNoMatch {
		t.Fatal(err)
	}
}

func TestOnlyOnePage(t *testing.T) {
	testFetches(t, "symphogear_live")
}
