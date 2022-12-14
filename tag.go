package pixiv

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"

	jsoniter "github.com/json-iterator/go"
)

type tag struct {
	pixiv
	tagName string
}

// Tag Constructor of tag of pictures.
//
// tagName -name of tag, can't be multi.
func Tag(tagName string) *tag {
	t := new(tag)
	t.rname = "tag"
	t.log = myLog.WithField("place", "tag")
	t.savePath = globalConfig.GetString("download.tag.path")
	t.fileDir = tagName
	t.tagName = tagName
	t.baseURL = "https://www.pixiv.net/ajax/search/artworks/%s"
	return t
}

// Num num of pictures to get.
func (t *tag) Num(num int) *tag {
	if num <= 0 {
		t.log.Fatalln("Please give a number > 0")
	}
	t.num = num
	return t
}

// Download download pictures to local
func (t *tag) Download() {
	if t.num == 0 {
		t.log.Fatalln("Please set num before download")
	}
	t.downLoadImg(t.getImgUrls(t.getIds()))
}

// Upload upload pictures to minio server
func (t *tag) Upload() {
	if t.num == 0 {
		t.log.Fatalln("Please set num before download")
	}
	t.upLoadImg(t.getImgUrls(t.getIds()))
}

func (t *tag) getIds() chan string {
	ids := make(chan string, t.num)
	pageUp := t.num/60 + 1
	numLeft := t.num % 60
	pageCount := pageUp
	// wait chan
	idsChan := make(chan int)

	URL := fmt.Sprintf(t.baseURL, t.tagName)
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		t.log.WithError(err).Fatalf("Fail to create request, URL=%s", URL)
	}
	setHeader(req)

	for ; pageUp > 0; pageUp-- {
		go func(pageUp int) {
			idNum := 60
			if pageUp == pageCount {
				idNum = numLeft
			}
			q := req.URL.Query()
			q.Add("word", t.tagName)
			q.Add("order", "date_d")
			q.Add("mode", "all")
			q.Add("p", strconv.Itoa(pageUp))
			q.Add("s_mode", "s_tag_full")
			q.Add("type", "all")
			q.Add("lang", "zh")
			req.URL.RawQuery = q.Encode()
			res, err := client.Do(req)
			if err != nil {
				t.log.WithError(err).Fatalln("Fail to get response")
			}

			if code := res.StatusCode; code != 200 {
				t.log.Fatalf("URL Code=%d", res.StatusCode)
			}

			reader, _ := gzip.NewReader(res.Body)
			content, err := io.ReadAll(reader)
			defer res.Body.Close()
			if err != nil {
				t.log.WithError(err).Fatalln("Fail to read response")
			}

			for ; idNum > 0; idNum-- {
				ids <- jsoniter.Get(content, "body").Get("illustManga").Get("data", idNum-1).Get("id").ToString()
			}
			idsChan <- 1

		}(pageUp)
	}
	go func(pageCount int, idsChan chan int) {
		for ; pageCount > 0; pageCount-- {
			<-idsChan
		}
		close(ids)
	}(pageCount, idsChan)
	return ids
}
