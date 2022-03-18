package main

import (
	"fmt"
	"io"
	"regexp"
	"os"
	"net/http"
	"encoding/json"
	"strings"
	"time"
	"path/filepath"
	"strconv"
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
		go downloader(queue, &wg, bar)
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
			coubDir := filepath.Join(dirName, strconv.Itoa(cb.Id))
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

func downloader(ch chan Task, wg *sync.WaitGroup, bar *progressbar.ProgressBar) {
	defer wg.Done()
	for t := range ch {
		download(t.C, t.DirName)
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

func download(c Coub, dirName string) {
	err := queryAndSaveResourceToFile(c.File_Versions.Html5.Video, dirName, "vi")
	if err != nil {
		panic(fmt.Errorf("while processing coub %n: %w", c.Id, err))
	}
	if c.File_Versions.Html5.Audio != nil {
		err = queryAndSaveResourceToFile(*c.File_Versions.Html5.Audio, dirName, "au")
		if err != nil {
			panic(fmt.Errorf("while processing coub %n: %w", c.Id, err))
		}
	}
}

func queryAndSaveResourceToFile(res CoubHTML5Resource, dirName string, fname string) error {
	u := getUrl(res)
	if u == "" {
		return errors.New("resource not found")
	}
	queryAndSaveToFile(u, dirName, fname)
	return nil
}

func queryAndSaveToFile(url string, dirName string, fname string) {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		panic(resp)
	}

	err = os.MkdirAll(dirName, 0775)
	if err != nil {
		panic(err)
	}

	parts := strings.Split(url, ".")
	ext := "." + parts[len(parts) - 1]
	f, err := os.Create(filepath.Join(dirName, fname + ext))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	f.ReadFrom(resp.Body)
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
