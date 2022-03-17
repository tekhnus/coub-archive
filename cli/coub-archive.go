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
)

func main() {
	curlfile := os.Args[1]
	input, err := os.ReadFile(curlfile)
	if err != nil {
		panic(err)
	}
	inputS := (string)(input)
	r := regexp.MustCompile(`-H 'Cookie: (.*)'`)
	matches := r.FindStringSubmatch(inputS)
	if len(matches) <=1 {
		panic("wrong curl string")
	}
	cookie := matches[1]
	page := 1
	dirTag := "coub-archive-" + time.Now().Format("2006-01-02T15_04_05")
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	dirName := filepath.Join(filepath.Dir(exePath), dirTag)
	absDir, err := filepath.Abs(dirName)
	if err != nil {
		panic(err)
	}
	fmt.Println("saving to", absDir)
	bar := progressbar.Default(-1)
	queue := make(chan Task, 32)
	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go downloader(queue, &wg, bar)
	}
	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("https://coub.com/api/v2/timeline/likes?page=%d&per_page=25", page), nil)
		if err != nil {
			panic(err)
		}
		req.Header.Add("Cookie", cookie)
		var client http.Client
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			panic(resp)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		var firstPage Timeline
		err = json.Unmarshal(body, &firstPage)
		if err != nil {
			panic(err)
		}
		for _, cb := range firstPage.Coubs {
			coubDir := filepath.Join(dirName, strconv.Itoa(cb.Id))
			queue <- Task{cb, coubDir}
		}
		totalPages := firstPage.Total_Pages
		if page == totalPages {
			break
		}
		page += 1
	}
	close(queue)
	wg.Wait()
}

func downloader(ch chan Task, wg *sync.WaitGroup, bar *progressbar.ProgressBar) {
	defer wg.Done()
	for t := range ch {
		download(t.C, t.DirName)
		bar.Add(1)
	}
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
