package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

var defaultTimeout time.Duration

func main() {
	region := os.Getenv("REGION")
	if region == "" {
		log.Fatal("no region set")
	}

	defaultTimeout = 100*time.Millisecond
	var err error
	timeoutDuration := os.Getenv("TIMEOUT")
	if timeoutDuration != "" {
		defaultTimeout, err = time.ParseDuration(timeoutDuration)
		if err != nil {
			log.Fatalf("invalid default timeout %s", err.Error())
		}
	}

	log.Printf("using %s as default timeout", defaultTimeout)

	log.Printf("starting watchman region %s...", region)
	http.HandleFunc("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting port to %s", port)
	} 

	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	urls, ok := query["url"]
	if !ok || len(urls) == 0 {
		log.Print("missing url")
		w.WriteHeader(http.StatusBadRequest)
		return 
	}
	url := urls[0]

	var err error 
	timeout := defaultTimeout
	customTimeouts, ok := query["timeout"]
	if ok && len(customTimeouts) != 0 {
		timeout, err = time.ParseDuration(customTimeouts[0])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Print("bad timeout")
			w.Write([]byte("bad timeout"))
			return 
		}
	}

	log.Printf("checking %s... (timeout: %s)", url, timeout)
	ctx := context.Background()
	err = check(ctx, url, timeout)
	if err != nil {
		log.Print(err.Error())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(err.Error()))
	} else {
		w.WriteHeader(http.StatusOK)
		log.Print("success")
		w.Write([]byte("ok"))
	}
}

func check(ctx context.Context, url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
    	return err
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("non-ok response %s", resp.Status)
	}

	return nil 
}