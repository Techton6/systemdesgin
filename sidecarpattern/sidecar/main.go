package main
import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const logFile = "sidecar.log"

func main() {
	absPath, _ := filepath.Abs(logFile)
	fmt.Println("sidecar: started, watching (absolute path):", absPath)

	var offset int64 = 0
	lastHeartbeat := time.Now()

	for {
		f, err := os.Open(logFile)
		if err != nil {
			// main app may not have created the file yet
			if time.Since(lastHeartbeat) > 3*time.Second {
				fmt.Println("sidecar: still waiting — file not found yet at", absPath)
				lastHeartbeat = time.Now()
			}
			time.Sleep(300 * time.Millisecond)
			continue
		}

		info, err := f.Stat()
		if err != nil {
			f.Close()
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// file got smaller than what we've read -> it was rotated/truncated
		if info.Size() < offset {
			offset = 0
		}

		if info.Size() > offset {
			if _, err := f.Seek(offset, 0); err != nil {
				f.Close()
				time.Sleep(300 * time.Millisecond)
				continue
			}

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}
				printRequestTime(line)
			}
			offset = info.Size()
			lastHeartbeat = time.Now()
		} else if time.Since(lastHeartbeat) > 3*time.Second {
			fmt.Printf("sidecar: still watching, no new data (file size=%d, offset=%d)\n", info.Size(), offset)
			lastHeartbeat = time.Now()
		}

		f.Close()
		time.Sleep(200 * time.Millisecond) 
	}
}

func printRequestTime(line string) {
	parts := strings.SplitN(line, " | ", 2)
	timestamp := parts[0]

	if t, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
		fmt.Printf("sidecar: detected request at %s -> %s\n",
			t.Format("15:04:05.000"), line)
	} else {
		fmt.Println("sidecar: detected line ->", line)
	}
}