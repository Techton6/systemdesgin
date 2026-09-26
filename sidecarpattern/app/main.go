package main

import (
	"fmt"

	"math/rand"
	"net/http"
	"path/filepath"
	"os"
	"time"
)

const (
	logFile   = "sidecar.log"
	targetURL = "https://github.com"
)

func main() {
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("main-app: failed to open log file:", err)
		os.Exit(1)
	}
	defer f.Close()
 
	client := &http.Client{Timeout: 5 * time.Second}
 
	absPath, _ := filepath.Abs(logFile)
	fmt.Println("main-app: started, will send requests to", targetURL)
	fmt.Println("main-app: writing logs to (absolute path):", absPath)
 
	for {
		// random delay between 1 and 5 seconds before the next request
		delay := time.Duration(rand.Intn(4000)+1000) * time.Millisecond
		time.Sleep(delay)
 
		sentAt := time.Now()
		resp, err := client.Get(targetURL)
 
		status := "ERROR"
		if err != nil {
			status = err.Error()
		} else {
			status = resp.Status
			resp.Body.Close()
		}
 
		line := fmt.Sprintf("%s | delay=%s | url=%s | status=%s\n",
			sentAt.Format(time.RFC3339Nano), delay, targetURL, status)
 
		if _, werr := f.WriteString(line); werr != nil {
			fmt.Println("main-app: failed to write log line:", werr)
			continue
		}

		f.Sync()
 
		fmt.Print("main-app: logged -> ", line)
	}
}
 