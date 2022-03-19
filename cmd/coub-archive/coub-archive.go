package main

import (
	"fmt"
	"io"
	"regexp"
	"os"
	"net/http"
	"encoding/json"
	"strconv"
	"time"
	"path"
	"path/filepath"
	"sync"
	"errors"
	"github.com/schollz/progressbar/v3"
	"log"
)

func main() {
	err := doMain()
	if err != nil {
		log.Fatal(err)
	}
}

func doMain() error {
	var wg sync.WaitGroup
	errchan := make(chan error)
	wg.Add(1)
	go reportErrors(errchan, &wg)
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	var curlfile string
	if len(os.Args) > 1 {
		curlfile = os.Args[1]
	} else {
		curlfile = filepath.Join(filepath.Dir(exePath), "coub-curl.txt")
	}
	cookie, err := readCookies(curlfile)
	if err != nil {
		return err
	}
	dirTag := "coub-archive-" + time.Now().Format("2006-01-02T15_04_05")
	dirName := filepath.Join(filepath.Dir(exePath), dirTag)
	absDir, err := filepath.Abs(dirName)
	if err != nil {
		return err
	}
	fmt.Println("saving to", absDir)
	cnt := 0
	bar := progressbar.Default(1)
	queue := make(chan Task, 64000)
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go downloader(queue, &wg, bar, func(item CoubMediaRequestResponse) {
			saveToFile(dirName, item)
		})
	}
	reqresp := make(chan TimelineRequestResponse)
	go paginateThroughTimeline(reqresp, errchan, "/timeline/likes?", cookie)

	for rr := range reqresp {
		firstPage := rr.Response
		cnt += len(firstPage.Coubs)
		bar.ChangeMax(cnt)
		for _, rawcb := range firstPage.Coubs {
			var cb Coub
			err := json.Unmarshal(rawcb, &cb)
			if err != nil {
				return err
			}
			err = saveCoubMetadata(filepath.Join(dirName, "timeline-likes", strconv.Itoa(rr.Response.Page), cb.Permalink), rawcb)
			if err != nil {
				return err
			}
			coubDir := filepath.Join(dirName, "media", cb.Permalink)
			queue <- Task{cb, coubDir}
		}
	}
	close(queue)
	close(errchan)
	wg.Wait()
	return nil
}

func reportErrors(errchan chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	for err := range errchan {
		log.Println(err)
	}
}

func readCookies(curlfile string) (string, error) {
	input, err := os.ReadFile(curlfile)
	if err != nil {
		return "", err
	}
	inputS := (string)(input)
	r := regexp.MustCompile(`-H 'Cookie: (.*)'`)
	matches := r.FindStringSubmatch(inputS)
	if len(matches) <=1 {
		return "", fmt.Errorf("something is wrong with cookie file")
	}
	cookie := matches[1]
	return cookie, nil
}

func saveCoubMetadata(dirName string, rawcb json.RawMessage) error {
	err := os.MkdirAll(dirName, 0775)
	if err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(dirName, "metadata.txt"))
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(rawcb)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func downloader(ch chan Task, wg *sync.WaitGroup, bar *progressbar.ProgressBar, callback func(CoubMediaRequestResponse)) {
	defer wg.Done()
	for t := range ch {
		res := download(t.C, t.DirName)
		callback(res)
		bar.Add(1)
	}
}

func paginateThroughTimeline(outp chan TimelineRequestResponse, errchan chan error, query string, cookies string) {
	defer close(outp)
	page := 1
	for {
		q := fmt.Sprintf("%spage=%d&per_page=25", query, page)
		body, err := performRequest(q, cookies)
		if err != nil {
			errchan <- err
			return
		}
		var firstPage TimelineResponse
		err = json.Unmarshal(body, &firstPage)
		if err != nil {
			errchan <- err
			return
		}
		outp <- TimelineRequestResponse{q, firstPage}
		totalPages := firstPage.Total_Pages
		if page == totalPages {
			break
		}
		page += 1
	}
}

func performRequest(query string, cookies string) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://coub.com/api/v2" + query, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Cookie", cookies)
	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coub.com response is not good: %s", resp)
	}
	return io.ReadAll(resp.Body)
}

func download(c Coub, dirName string) CoubMediaRequestResponse {
	videoUrl, videoB, err := queryAndSaveResourceToFile(c.File_Versions.Html5.Video, filepath.Join(dirName, "best-video"), "video")
	if err != nil {
		panic(fmt.Errorf("while processing coub %n: %w", c.Permalink, err))
	}
	audioUrl := ""
	var audioB []byte
	if c.File_Versions.Html5.Audio != nil {
		audioUrl, audioB, err = queryAndSaveResourceToFile(*c.File_Versions.Html5.Audio, filepath.Join(dirName, "best-audio"), "audio")
		if err != nil {
			panic(fmt.Errorf("while processing coub %n: %w", c.Permalink, err))
		}
	}
	return CoubMediaRequestResponse{c.Permalink, videoUrl, videoB, audioUrl, audioB}
}

func queryAndSaveResourceToFile(res CoubHTML5Resource, dirName string, fname string) (string, []byte, error) {
	u := getUrl(res)
	if u == "" {
		return "", nil, errors.New("resource not found")
	}
	b, err := queryAndSaveToFile(u, dirName, fname)
	return u, b, err
}

func queryAndSaveToFile(url string, dirName string, fname string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

func saveToFile(rootdir string, data CoubMediaRequestResponse) error {
	dirName := filepath.Join(rootdir, "media", data.CoubPermalink)

	err := saveBytesToFile(filepath.Join(dirName, "best-video", "video" + path.Ext(data.VideoRequest)), data.BestVideo)
	if err != nil {
		return err
	}
	err = saveBytesToFile(filepath.Join(dirName, "best-video", "request.txt"), ([]byte)(data.VideoRequest))
	if err != nil {
		return err
	}
	err = saveBytesToFile(filepath.Join(dirName, "best-audio", "audio" + path.Ext(data.AudioRequest)), data.BestAudio)
	if err != nil {
		return err
	}
	err = saveBytesToFile(filepath.Join(dirName, "best-audio", "request.txt"), ([]byte)(data.AudioRequest))
	if err != nil {
		return err
	}
	return nil
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
	Page int
	Total_Pages int
	Coubs []Coub
}

type Coub struct {
	Id int
	Permalink string
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
	High CoubHTML5Link
	Med CoubHTML5Link
}

type CoubHTML5Link struct {
	Url string
}

type Task struct {
	C Coub
	DirName string
}

type TimelineRequestResponse struct {
	Request string
	Response TimelineResponse
}

type TimelineResponse struct {
	Page int
	Total_Pages int
	Coubs []json.RawMessage
}

type CoubMediaRequestResponse struct {
	CoubPermalink string
	VideoRequest string
	BestVideo []byte
	AudioRequest string
	BestAudio []byte
}
