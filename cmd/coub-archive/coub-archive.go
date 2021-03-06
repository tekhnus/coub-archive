package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/schollz/progressbar/v3"
	shell "github.com/ipfs/go-ipfs-api"
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
	"github.com/ncruces/zenity"
)

var metadataClient http.Client
var mediaClient http.Client
var guiErrors bool

func main() {
	noguiFlag := flag.Bool("no-gui", false, "do not use the gui, read the flags")
	whatFlag := flag.String("what", "/timeline/likes", "api endpoint")
	ipfsFlag := flag.Bool("ipfs", false, "upload to ipfs")
	orderByFlag := flag.String("order-by", "", "field to order by")
	flag.Parse()
	sh := shell.NewShell("localhost:5001")
	var updProgress func(int, int)
	var success func()
	if !*noguiFlag {
		guiErrors = true
		err := zenity.Question("Save to IPFS?", zenity.CancelLabel("No"), zenity.Title("Choose the destination"))
		*ipfsFlag = err == nil

		if *ipfsFlag && !sh.IsUp() {
			terminateIfError(fmt.Errorf("cannot connect to IPFS app, make sure it is running"))
		}
		apiPaths := map[string]string{
			"my-likes": "/timeline/likes",
			"my-feed": "/timeline",
			"channel": "/timeline/channel",
			"tag": "/timeline/tag",
			"hot": "/timeline/hot",
			"coub-of-the-day": "/timeline/explore/coub_of_the_day",
		}
		var ks []string
		for k := range apiPaths {
			ks = append(ks, k)
		}
		res, err := zenity.List(
			"choose what to download",
			ks,
			zenity.DefaultItems("my-likes"),
			zenity.DisallowEmpty(),
			zenity.Title("Choose what to download"),
		)
		terminateIfError(err)
		if res == "" {
			res = "my-likes"
		}
		var ok bool
		*whatFlag, ok = apiPaths[res]
		if !ok {
			terminateIfError(fmt.Errorf("something wrong with the dialog"))
		}

		var orderOptions []string
		if *whatFlag == "/timeline/channel" {
			orderOptions = []string{"default", "likes_count", "views_count", "newest_popular"}
		} else if *whatFlag == "/timeline/tag" {
			orderOptions = []string{"default", "likes_count", "views_count", "newest_popular", "oldest"}
		} else if *whatFlag == "/timeline/hot" {
			orderOptions = []string{"default", "likes_count", "views_count", "newest_popular", "oldest"}
		}

		var clarify string
		if *whatFlag == "/timeline/channel" {
			clarify = "Enter channel name (from its URL):"
		} else if *whatFlag == "/timeline/tag" {
			clarify = "Enter tag name without the '#':"
		}
		if clarify != "" {
			res, err := zenity.Entry(clarify, zenity.Title("input"))
			terminateIfError(err)
			*whatFlag += "/" + res
		}

		if len(orderOptions) != 0 {
			res, err := zenity.List(
				"choose the order",
				orderOptions,
				zenity.DefaultItems("default"),
				zenity.DisallowEmpty(),
				zenity.Title("download order"),
			)
			terminateIfError(err)
			if res == "" {
				res = "default"
			}
			if res != "default" {
				*orderByFlag = res
			}
		}
		updProgress = guiProgressBar()
		success = guiSuccess
	} else {
		updProgress = progressBar()
		success = func() {
			fmt.Println("\nsuccess!")
		}
	}
	rootdir, err := os.UserHomeDir()
	terminateIfError(err)
	temproot := filepath.Join(rootdir, "coubs-temporary-folder")
	err = os.MkdirAll(temproot, 0775)
	terminateIfError(err)
	defer os.RemoveAll(temproot)
	queryId := time.Now().Format("2006-01-02T15_04_05") + strings.ReplaceAll(*whatFlag, "/", "_") + "_" + *orderByFlag
	var mu sync.Mutex
	saveMetadata := func(rr TimelineRequestResponse) error {
		mu.Lock()
		defer mu.Unlock()
		if *ipfsFlag {
			return saveMetaToIPFS(sh, temproot, queryId, rr);
		}
		return saveMetaToFile(rootdir, temproot, queryId, rr);
	}
	saveMedia := func(tl TimelineRequestResponse, item CoubMediaRequestResponse) error {
		mu.Lock()
		defer mu.Unlock()
		if *ipfsFlag {
			return saveMediaToIPFS(sh, temproot, queryId, tl, item);
		}
		return saveMediaToFile(rootdir, temproot, queryId, tl, item)
	}
	err = doTimeline(saveMetadata, saveMedia, updProgress, *whatFlag, *orderByFlag)
	terminateIfError(err)
	success()
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

func guiProgressBar() func(int, int) {
	dlg, err := zenity.Progress()
	terminateIfError(err)
	dlg.Text("starting downloading...")

	done := 0
	total := 0

	return func(deltaDone int, deltaTotal int) {
		done += deltaDone
		total += deltaTotal
		err := dlg.Value(100 * done / total)
		terminateIfError(err)
		err = dlg.Text(fmt.Sprintf("%d from %d coubs downloaded", done, total))
		terminateIfError(err)
	}
}

func guiSuccess() {
	zenity.Info("success!", zenity.Title("yay!"))
}

func terminateIfError(err error) {
	if err == nil {
		return
	}
	if guiErrors {
		zenity.Error(err.Error(), zenity.Title("error"))
	}
	log.Fatal(err)
}

func doTimeline(saveMetadata func(TimelineRequestResponse) error, saveMedia func(TimelineRequestResponse, CoubMediaRequestResponse) error, updProgress func(int, int), apiPath string, order_by string) error {
	var headers map[string]string
	if apiPath == "/timeline/likes" || apiPath == "/timeline" {
		var err error
		headers, err = getAuthHeaders()
		if err != nil {
			return err
		}
	}

	var params []string
	if order_by != "" {
		params = append(params, "order_by=" + order_by)
	}

	return doTimelineImpl(saveMetadata, saveMedia, apiPath, params, headers, updProgress)
}

func getAuthHeaders() (map[string]string, error) {
	exePath, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	curlfile := filepath.Join(exePath, "coub-curl.txt")
	cookie, err := readCookies(curlfile)
	if err != nil {
		return nil, err
	}
	return map[string]string{"Cookie": cookie}, nil
}

func doTimelineImpl(saveMetadata func(TimelineRequestResponse) error, saveMedia func(TimelineRequestResponse, CoubMediaRequestResponse) error, apiPath string, params []string, headers map[string]string, updProgress func(int, int)) error {
	queue := make(chan MediaRequest, 64000)
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
				queue <- MediaRequest{cb, rr}
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
			err := mediaDownloader(queue, func(tl TimelineRequestResponse, item CoubMediaRequestResponse) error {
				err := saveMedia(tl, item)
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

func saveMetaToFile(rootdir string, temproot string, queryId string, data TimelineRequestResponse) error {
	dirName, err := os.MkdirTemp(temproot, "coub-archive-temporary-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dirName)
	err = saveMetaToStash(dirName, data)
	if err != nil {
		return err
	}
	target := filepath.Join(rootdir, "coubs", queryId, "pages", fmt.Sprintf("%03d", data.Response.Page))
	err = os.MkdirAll(filepath.Dir(target), 0775)
	if err != nil {
		return err
	}
	return os.Rename(dirName, target)
}

func saveMetaToIPFS(sh *shell.Shell, temproot string, queryId string, data TimelineRequestResponse) error {
	dirName, err := os.MkdirTemp(temproot, "coub-archive-temporary-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dirName)
	err = saveMetaToStash(dirName, data)
	if err != nil {
		return err
	}
	return copyStashToIPFS(sh, dirName, path.Join("/coubs", queryId, "pages", fmt.Sprintf("%03d", data.Response.Page)))
}

func saveMetaToStash(dirName string, data TimelineRequestResponse) error {
	saveBytesToFile(filepath.Join(dirName, "request.txt"), ([]byte)(data.Request))
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
		err = saveBytesToFile(filepath.Join(dirName, "coubs", cb.Permalink, "meta.txt"), b)
		if err != nil {
			return err
		}
	}
	return nil
}

func saveMediaToFile(rootdir string, temproot string, queryId string, tl TimelineRequestResponse, data CoubMediaRequestResponse) error {
	dirName, err := os.MkdirTemp(temproot, "coub-archive-temporary-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dirName)
	err = saveMediaToStash(dirName, data)
	if err != nil {
		return err
	}
	target := filepath.Join(rootdir, "coubs", queryId, "pages", fmt.Sprintf("%03d", tl.Response.Page), "coubs", data.CoubPermalink, "media")
	err = os.MkdirAll(filepath.Dir(target), 0775)
	if err != nil {
		return err
	}
	return os.Rename(dirName, target)
}

func saveMediaToIPFS(sh *shell.Shell, temproot string, queryId string, tl TimelineRequestResponse, data CoubMediaRequestResponse) error {
	dirName, err := os.MkdirTemp(temproot, "coub-archive-temporary-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dirName)
	err = saveMediaToStash(dirName, data)
	if err != nil {
		return err
	}
	return copyStashToIPFS(sh, dirName, path.Join("/coubs", queryId, "pages", fmt.Sprintf("%03d", tl.Response.Page), "coubs", data.CoubPermalink, "media"))
}

func saveMediaToStash(dirName string, data CoubMediaRequestResponse) error {
	err := saveBytesToFile(filepath.Join(dirName, "best-video"+path.Ext(data.VideoRequest)), data.BestVideo)
	if err != nil {
		return err
	}
	err = saveBytesToFile(filepath.Join(dirName, "best-video-request.txt"), ([]byte)(data.VideoRequest))
	if err != nil {
		return err
	}
	if data.BestAudio != nil {
		err = saveBytesToFile(filepath.Join(dirName, "best-audio"+path.Ext(data.AudioRequest)), data.BestAudio)
		if err != nil {
			return err
		}
		err = saveBytesToFile(filepath.Join(dirName, "best-audio-request.txt"), ([]byte)(data.AudioRequest))
		if err != nil {
			return err
		}
	}
	return nil
}

func copyStashToIPFS(sh *shell.Shell, dirName string, target string) error {
	err := ipfsFilesMkdirParents(sh, context.Background(), path.Dir(target))
	if err != nil {
		return err
	}
	res, err := sh.AddDir(dirName)
	if err != nil {
		return err
	}
	defer sh.Unpin(res)
	err = sh.FilesCp(context.Background(), path.Join("/ipfs", res), target)
	if err != nil {
		return err
	}
	return nil
}

func ipfsFilesMkdirParents(sh *shell.Shell, ctx context.Context, dir string) error {
	if dir == "/" {
		return nil
	}
	par := path.Dir(dir)
	err := ipfsFilesMkdirParents(sh, ctx, par)
	if err != nil {
		return err
	}
	err = sh.FilesMkdir(ctx, dir)
	if err == nil {
		return nil
	}
	stat, err := sh.FilesStat(ctx, dir)
	if err != nil {
		return err
	}
	if stat.Type != "directory" {
		return fmt.Errorf("tried to make directory %s, but it is a %s", dir, stat.Type)
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

func mediaDownloader(ch chan MediaRequest, callback func(TimelineRequestResponse, CoubMediaRequestResponse) error) error {
	for mr := range ch {
		coub := mr.Cb
		res, err := downloadMedia(coub)
		if err != nil {
			return err
		}
		err = callback(mr.TimelineRequest, res)
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
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coub.com response is not good: %s %s", resp.Status, query)
	}
	return res, nil
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
	var err error
	for i := 1; i < 3; i++ {
		if i > 1 {
			time.Sleep(60 * time.Second)
		}
		var req *http.Request
		req, err = http.NewRequest("GET", url, nil)
		req.Close = true
		if err != nil {
			continue
		}
		resp, err := mediaClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("error while querying the resource: %s %s", resp.Status, url)
			continue
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		return b, nil
	}
	return nil, err
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

type MediaRequest struct {
	Cb              Coub
	TimelineRequest TimelineRequestResponse
}
