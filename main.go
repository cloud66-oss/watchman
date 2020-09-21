package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	region := os.Getenv("REGION")
	if region == "" {
		log.Fatal("no region set")
	}

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

	log.Printf("checking %s...", url)
	ctx := context.Background()
	err := check(ctx, url)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(err.Error()))
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func check(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
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