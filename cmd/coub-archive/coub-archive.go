package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var metadataClient http.Client
var mediaClient http.Client

func main() {
	updProgress := progressBar()
	err := doTimelineLikes(updProgress)
	terminateIfError(err)
}

func progressBar() func(int, int) {
	bar := progressbar.Default(1)
	total := 1

	return func(deltaDone int, deltaTotal int) {
		total += deltaTotal
		bar.Add(deltaDone)
		bar.ChangeMax(total)
	}
}

func terminateIfError(err error) {
	if err == nil {
		return
	}
	log.Fatal(err)
}

func doTimelineLikes(updProgress func(int, int)) error {
	headers, err := getAuthHeaders()
	if err != nil {
		return err
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	dirTag := "coubs"
	dirName := filepath.Join(filepath.Dir(exePath), dirTag)
	queryId := time.Now().Format("2006-01-02T15_04_05")
	saveMetadata := func(rr TimelineRequestResponse) error {
		return saveMetadataToFile(dirName, "timeline-likes", queryId, rr);
	}
	return doTimeline(saveMetadata, "/timeline/likes", []string{}, headers, updProgress)
}

func getAuthHeaders() (map[string]string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	curlfile := filepath.Join(filepath.Dir(exePath), "coub-curl.txt")
	cookie, err := readCookies(curlfile)
	if err != nil {
		return nil, err
	}
	return map[string]string{"Cookie": cookie}, nil
}

func doTimeline(saveMetadata func(TimelineRequestResponse) error, apiPath string, params []string, headers map[string]string, updProgress func(int, int)) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	dirTag := "coubs"
	dirName := filepath.Join(filepath.Dir(exePath), dirTag)

	queue := make(chan Coub, 64000)
	go func() {
		defer close(queue)
		err := paginateThroughTimeline(apiPath, params, headers, func(rr TimelineRequestResponse) error {
			err := saveMetadata(rr)
			if err != nil {
				return err
			}
			firstPage := rr.Response
			for _, rawcb := range firstPage.Coubs {
				var cb Coub
				err := json.Unmarshal(rawcb, &cb)
				if err != nil {
					return err
				}
				updProgress(+0, +1)
				queue <- cb
			}
			return nil
		})
		terminateIfError(err)
	}()

	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := mediaDownloader(queue, func(item CoubMediaRequestResponse) error {
				err := saveMediaToFile(dirName, item)
				if err != nil {
					return err
				}
				updProgress(+1, +0)
				return nil
			})
			terminateIfError(err)
		}()
	}
	wg.Wait()

	return nil
}

func saveMetadataToFile(rootdir string, topic string, queryId string, data TimelineRequestResponse) error {
	pageRoot := filepath.Join(rootdir, topic, queryId, fmt.Sprintf("%03d", data.Response.Page))
	saveBytesToFile(filepath.Join(pageRoot, "request.txt"), ([]byte)(data.Request))
	page := data.Response
	for _, rawcb := range page.Coubs {
		var cb Coub
		err := json.Unmarshal(rawcb, &cb)
		if err != nil {
			return err
		}
		b, err := json.Marshal(rawcb)
		if err != nil {
			return err
		}
		err = saveBytesToFile(filepath.Join(pageRoot, cb.Permalink, "metadata.txt"), b)
		if err != nil {
			return err
		}
	}
	return nil
}

func saveMediaToFile(rootdir string, data CoubMediaRequestResponse) error {
	dirName := filepath.Join(rootdir, "media", data.CoubPermalink)

	err := saveBytesToFile(filepath.Join(dirName, "best-video", "video"+path.Ext(data.VideoRequest)), data.BestVideo)
	if err != nil {
		return err
	}
	err = saveBytesToFile(filepath.Join(dirName, "best-video", "request.txt"), ([]byte)(data.VideoRequest))
	if err != nil {
		return err
	}
	if data.BestAudio != nil {
		err = saveBytesToFile(filepath.Join(dirName, "best-audio", "audio"+path.Ext(data.AudioRequest)), data.BestAudio)
		if err != nil {
			return err
		}
		err = saveBytesToFile(filepath.Join(dirName, "best-audio", "request.txt"), ([]byte)(data.AudioRequest))
		if err != nil {
			return err
		}
	}
	return nil
}

func readCookies(curlfile string) (string, error) {
	input, err := os.ReadFile(curlfile)
	if err != nil {
		return "", err
	}
	inputS := (string)(input)
	r := regexp.MustCompile(`-H 'Cookie: (.*)'`)
	matches := r.FindStringSubmatch(inputS)
	if len(matches) <= 1 {
		return "", fmt.Errorf("something is wrong with cookie file")
	}
	cookie := matches[1]
	return cookie, nil
}

func mediaDownloader(ch chan Coub, callback func(CoubMediaRequestResponse) error) error {
	for coub := range ch {
		res, err := downloadMedia(coub)
		if err != nil {
			return err
		}
		err = callback(res)
		if err != nil {
			return err
		}
	}
	return nil
}

func paginateThroughTimeline(query string, params []string, headers map[string]string, callback func(TimelineRequestResponse) error) error {
	page := 1
	for {
		extParams := append(params, fmt.Sprintf("page=%d", page), "per_page=25")
		q := query + "?" + strings.Join(extParams, "&")
		body, err := performRequest(q, headers)
		if err != nil {
			return err
		}
		var firstPage TimelineResponse
		err = json.Unmarshal(body, &firstPage)
		if err != nil {
			return err
		}
		err = callback(TimelineRequestResponse{q, firstPage})
		if err != nil {
			return err
		}
		totalPages := firstPage.Total_Pages
		if page == totalPages {
			break
		}
		page += 1
	}
	return nil
}

func performRequest(query string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://coub.com/api/v2"+query, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	resp, err := metadataClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coub.com response is not good: %s", resp)
	}
	return io.ReadAll(resp.Body)
}

func downloadMedia(c Coub) (CoubMediaRequestResponse, error) {
	videoUrl, videoB, err := downloadResource(c.File_Versions.Html5.Video)
	if err != nil {
		return CoubMediaRequestResponse{}, fmt.Errorf("while processing coub %s: %w", c.Permalink, err)
	}
	audioUrl := ""
	var audioB []byte
	if c.File_Versions.Html5.Audio != nil {
		audioUrl, audioB, err = downloadResource(*c.File_Versions.Html5.Audio)
		if err != nil {
			return CoubMediaRequestResponse{}, fmt.Errorf("while processing coub %s: %w", c.Permalink, err)
		}
	}
	return CoubMediaRequestResponse{c.Permalink, videoUrl, videoB, audioUrl, audioB}, nil
}

func downloadResource(res CoubHTML5Resource) (string, []byte, error) {
	u := getUrl(res)
	if u == "" {
		return "", nil, errors.New("resource not found")
	}
	b, err := downloadFromUrl(u)
	return u, b, err
}

func downloadFromUrl(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	req.Close = true
	if err != nil {
		return nil, err
	}
	resp, err := mediaClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

func saveBytesToFile(path string, b []byte) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0775)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	f.Write(b)
	return nil
}

func getUrl(res CoubHTML5Resource) string {
	if res.Higher.Url != "" {
		return res.Higher.Url
	}
	if res.High.Url != "" {
		return res.High.Url
	}
	if res.Med.Url != "" {
		return res.Med.Url
	}
	return ""
}

type Timeline struct {
	Page        int
	Total_Pages int
	Coubs       []Coub
}

type Coub struct {
	Id            int
	Permalink     string
	File_Versions CoubVersions
}

type CoubVersions struct {
	Html5 CoubHTML5
}

type CoubHTML5 struct {
	Video CoubHTML5Resource
	Audio *CoubHTML5Resource
}

type CoubHTML5Resource struct {
	Higher CoubHTML5Link
	High   CoubHTML5Link
	Med    CoubHTML5Link
}

type CoubHTML5Link struct {
	Url string
}

type TimelineRequestResponse struct {
	Request  string
	Response TimelineResponse
}

type TimelineResponse struct {
	Page        int
	Total_Pages int
	Coubs       []json.RawMessage
}

type CoubMediaRequestResponse struct {
	CoubPermalink string
	VideoRequest  string
	BestVideo     []byte
	AudioRequest  string
	BestAudio     []byte
}
