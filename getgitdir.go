package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	// "log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const GithubAPIToken = `b871a595ab544cc242ea526c9e70b8139799b726`

// var Message msg

const logo = `
██████╗ ███████╗████████╗     ██████╗ ██╗████████╗    ██████╗ ██╗██████╗ 
██╔════╝ ██╔════╝╚══██╔══╝    ██╔════╝ ██║╚══██╔══╝    ██╔══██╗██║██╔══██╗
██║  ███╗█████╗     ██║       ██║  ███╗██║   ██║       ██║  ██║██║██████╔╝
██║   ██║██╔══╝     ██║       ██║   ██║██║   ██║       ██║  ██║██║██╔══██╗
╚██████╔╝███████╗   ██║       ╚██████╔╝██║   ██║       ██████╔╝██║██║  ██║
╚═════╝ ╚══════╝   ╚═╝        ╚═════╝ ╚═╝   ╚═╝       ╚═════╝ ╚═╝╚═╝  ╚═╝																		 
`

var showLogo = true

// Creating api url from user input
func createApiURL(url string) (string, string) {
	downloadDir := regexp.MustCompile(`/tree/master/(.*)`).FindAllStringSubmatch(url, -1)
	if downloadDir == nil {
		panic("It seems you are either passing wrong url or trying to download whole repo.\nPlease use 'git clone' for cloning whole repo.\nSee help by -> $ getgitdir help\n")
	}
	branch := regexp.MustCompile(`/tree/(.*?)/`).FindAllStringSubmatch(url, -1)
	apiURL := strings.ReplaceAll(url, "https://github.com", "https://api.github.com/repos")
	apiURL = regexp.MustCompile("/tree/.*?/").ReplaceAllString(apiURL, "/contents/")
	apiURL = apiURL + "?ref=" + branch[0][1] + "&access_token=" + GithubAPIToken
	// fmt.Println(apiURL)
	return apiURL, downloadDir[0][1]
}

// Struct for unmarshalling data from json response.
type apiContentResp struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	URL         string `json:"url"`
	Size        string `json:"size"`
	HTMLURL     string `json:"html_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
	Message     string `json:"message"`
}

// Info is a struct for total files discovered and downloaded.
type Info struct {
	sync.Mutex
	TotalFilesDiscovered uint64
	TotalFilesDownloaded uint64
}

// AtomicIncrementDisc Atomically/Safely mutating totalfiles discovered
func (info *Info) AtomicIncrementDisc() uint64 {
	info.Lock()
	info.TotalFilesDiscovered++
	info.Unlock()
	return info.TotalFilesDiscovered
}

// Atomically/Safely mutating totalfiles downloaded
func (info *Info) AtomicIncrementDownld() uint64 {
	info.Lock()
	info.TotalFilesDownloaded++
	info.Unlock()
	return info.TotalFilesDownloaded
}

// Results defines channels which yield results for a total files discovered.
type Results struct {
	Inf chan Info
	Err chan error
}

func (r *Results) Read() {
	for {
		select {
		case info := <-r.Inf:
			fmt.Printf("\rTotal Files Discovered: %v Total Files Downloaded: %v", info.TotalFilesDiscovered, info.TotalFilesDownloaded)
		case err := <-r.Err:
			fmt.Println("e", err)
		}
	}
}

func (r *Results) Close() error {
	close(r.Inf)
	close(r.Err)
	return nil
}

// Helper Function for creating and returning a struct
func NewResults() *Results {
	return &Results{
		Inf: make(chan Info, 1),
		Err: make(chan error, 1),
	}
}

// Recursive download function for creating directories and downloading the files into them.
func download(wg *sync.WaitGroup, repoURL string, info *Info, results *Results) {
	// This function will recursively download a directory.
	// If there are nested directories then this function would be called for each directory.
	// All the files will be downloaded concurrently.
	defer wg.Done()

	jsonResp, err := http.Get(repoURL)
	if err != nil {
		results.Err <- err
	}

	body, err := ioutil.ReadAll(jsonResp.Body)

	if err != nil {
		results.Err <- err
	}

	jsonResp.Body.Close()

	var contents []apiContentResp
	json.Unmarshal([]byte(body), &contents)

	for _, cont := range contents {
		if cont.Type == "file" {

			info.AtomicIncrementDisc()
			results.Inf <- *info
			// downloading a file and saving it on proper path
			resp, err := http.Get(cont.DownloadURL)
			if err != nil {
				results.Err <- err
			}

			out, err := os.Create(cont.Path)
			if err != nil {
				results.Err <- err
			}

			_, err = io.Copy(out, resp.Body)
			if err != nil {
				results.Err <- err
			}
			info.AtomicIncrementDownld()

			resp.Body.Close()
			out.Close()
			results.Inf <- *info

		} else if cont.Type == "dir" {
			err := os.Mkdir(cont.Path, os.ModePerm)
			if err != nil {
				panic(err.Error())
			}
			wg.Add(1)
			go download(wg, cont.URL+"&access_token="+GithubAPIToken, info, results)
		}
	}
}

func showHelp() {
	if showLogo {
		fmt.Printf(logo + "\n")

	}
	fmt.Print("DESCRIPTION:\n\tUtility for downloading a directory from a github repository.\n\n")
	fmt.Print("USAGE:\n\tgetgitdir command [url]\n\n")
	fmt.Print("VERSION:\n\t1.0.0\n\n")
	fmt.Print("COMMANDS:\n\thelp, h Show a list of commands available and usage help.\n\n")
	fmt.Print("EXAMPLE USAGE:\n\t$ getgitdir https://github.com/bharat-rajani/getgitdir/tree/master/example\n\n")
}

func parseArgs(args []string) {

	if len(args) > 2 {
		unknownArg := ""
		if args[1] == "h" || args[1] == "help" {
			showHelp()
			unknownArg = args[2]
		} else {
			unknownArg = args[1]
		}
		panic(fmt.Sprintf("Unkown arguments %v ....", unknownArg))
	}

	if len(args) == 1 || args[1] == "h" || args[1] == "help" {
		showHelp()
		return
	}

	startDownloading(args[1])

}

func startDownloading(url string) {
	apiURL, downloadDir := createApiURL(url)
	os.RemoveAll(downloadDir)

	err := os.MkdirAll(downloadDir, os.ModePerm)
	if err != nil {
		panic(err.Error())
	}

	var wg sync.WaitGroup

	results := NewResults()
	defer results.Close()

	info := new(Info)

	startTime := time.Now()
	wg.Add(1)
	go download(&wg, apiURL, info, results)
	go results.Read()
	wg.Wait()
	elapsed := time.Since(startTime)
	fmt.Printf("\rTotal Files Discovered: %v Total Files Downloaded: %v", info.TotalFilesDiscovered, info.TotalFilesDownloaded)
	fmt.Printf("\nTime elapsed: %v\n", elapsed)
}

func main() {
	parseArgs(os.Args)
	// url := "https://github.com/oracle/container-images/tree/f15c4cdc4a1483e541eb00d18c8058d2b5988fff/7-slim"
	// if showLogo {
	// 	fmt.Printf(logo + "\n")

	// }
	// startDownloading(url)

}
