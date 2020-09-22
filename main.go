package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"golang.org/x/net/http2"
)

// CheckRequest is the payload for the service
type CheckRequest struct {
	Timeout           string `json:"timeout"`
	URL               string `json:"url"`
	RedirectsToFollow int    `json:"redirects_to_follow"`
	VerifyCerts       bool   `json:"verify_certs"`

	timeout time.Duration
}

// CheckResponse holds a single check's response
type CheckResponse struct {
	StatusCode       int           `json:"status"`
	Error            string        `json:"error"`
	DNSLookup        time.Duration `json:"dns_lookup"`
	TCPConnection    time.Duration `json:"tcp_connection"`
	TLSHandshake     time.Duration `json:"tls_handshake"`
	ServerProcessing time.Duration `json:"server_processing"`
	ContentTransfer  time.Duration `json:"content_transfer"`
	NameLookup       time.Duration `json:"name_lookup"`
	Connect          time.Duration `json:"connect"`
	PreTransfer      time.Duration `json:"pre_transfer"`
	StartTransfer    time.Duration `json:"start_transfer"`
	Total            time.Duration `json:"total"`
}

var (
	defaultTimeout      time.Duration
	defaultMaxRedirects int
	// Version holds the current version of watchman
	Version             string = "dev"
)

func main() {
	if os.Getenv("SENTRY_API") != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn: os.Getenv("SENTRY_API"),
		})
		if err != nil {
			log.Fatalf("sentry.Init: %s", err)
		}
		defer sentry.Flush(2 * time.Second)
	}

	var err error 
	defaultTimeout = 100 * time.Millisecond
	timeoutDuration := os.Getenv("TIMEOUT")
	if timeoutDuration != "" {
		defaultTimeout, err = time.ParseDuration(timeoutDuration)
		if err != nil {
			log.Fatalf("invalid default timeout %s", err.Error())
		}
	}

	defaultMaxRedirects = 3
	maxRedirects := os.Getenv("MAX_REDIRECTS")
	if maxRedirects != "" {
		defaultMaxRedirects, err = strconv.Atoi(maxRedirects)
		if err != nil {
			log.Fatalf("invalid max redirects %s", err.Error())
		}
	}

	log.Printf("using %s as default timeout", defaultTimeout)

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
	log.Print("request received")
	var request CheckRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var timeout time.Duration
	if request.Timeout != "" {
		timeout, err = time.ParseDuration(request.Timeout)
		if err != nil {
			http.Error(w, "bad timeout", http.StatusBadRequest)
			return
		}
		request.timeout = timeout
	} else {
		request.timeout = defaultTimeout
	}

	if request.RedirectsToFollow == 0 {
		request.RedirectsToFollow = defaultMaxRedirects
	}

	log.Printf("checking %v...", request)
	ctx := context.Background()
	response, err := check(ctx, request, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func isRedirect(resp *http.Response) bool {
	return resp.StatusCode > 299 && resp.StatusCode < 400
}

// readResponseBody consumes the body of the response.
// readResponseBody returns an informational message about the
// disposition of the response body's contents.
func readResponseBody(req *http.Request, resp *http.Response) error {
	if isRedirect(resp) {
		return nil
	}

	if _, err := io.Copy(ioutil.Discard, resp.Body); err != nil {
		return err
	}

	return nil
}

func check(ctx context.Context, request CheckRequest, redirectsFollowed int) (*CheckResponse, error) {
	url, err := url.Parse(request.URL)
	if err != nil {
		return nil, err
	}

	response := &CheckResponse{}

	var t0, t1, t2, t3, t4, t5, t6 time.Time
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) { t0 = time.Now() },
		DNSDone:  func(_ httptrace.DNSDoneInfo) { t1 = time.Now() },
		ConnectStart: func(_, _ string) {
			if t1.IsZero() {
				// connecting to IP
				t1 = time.Now()
			}
		},
		ConnectDone: func(net, addr string, err error) {
			if err != nil {
				response.Error = err.Error()
			}
			t2 = time.Now()
		},
		GotConn:              func(_ httptrace.GotConnInfo) { t3 = time.Now() },
		GotFirstResponseByte: func() { t4 = time.Now() },
		TLSHandshakeStart:    func() { t5 = time.Now() },
		TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { t6 = time.Now() },
	}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		response.Error = err.Error()
		return response, nil
	}

	ctx, cancel := context.WithTimeout(httptrace.WithClientTrace(context.Background(), trace), request.timeout)
	defer cancel()

	req = req.WithContext(ctx)

	tr := &http.Transport{
		MaxIdleConns:          5,
		IdleConnTimeout:       1 * time.Second,
		TLSHandshakeTimeout:   1 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if url.Scheme == "https" {
		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
		}

		tr.TLSClientConfig = &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: request.VerifyCerts,
		}

		// Because we create a custom TLSClientConfig, we have to opt-in to HTTP/2.
		// See https://github.com/golang/go/issues/14275
		err = http2.ConfigureTransport(tr)
		if err != nil {
			return nil, err
		}
	}

	client := &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// always refuse to follow redirects, check does that
			// manually if required.
			return http.ErrUseLastResponse
		},
	}

	req.Header.Set("User-Agent", fmt.Sprintf("watchman-%s", Version))
	resp, err := client.Do(req)
	if err != nil {
		response.Error = err.Error()
		return response, nil
	}

	_ = readResponseBody(req, resp)
	resp.Body.Close()
	response.StatusCode = resp.StatusCode

	t7 := time.Now()
	if t0.IsZero() {
		// we skipped DNS
		t0 = t1
	}

	switch url.Scheme {
	case "https":
		response.DNSLookup = t1.Sub(t0)
		response.TCPConnection = t2.Sub(t1)
		response.TLSHandshake = t6.Sub(t5)
		response.ServerProcessing = t4.Sub(t3)
		response.ContentTransfer = t7.Sub(t4)
		response.NameLookup = t1.Sub(t0)
		response.Connect = t2.Sub(t0)
		response.PreTransfer = t3.Sub(t0)
		response.StartTransfer = t4.Sub(t0)
		response.Total = t7.Sub(t0)
	case "http":
		response.DNSLookup = t1.Sub(t0)
		response.TCPConnection = t3.Sub(t1)
		response.ServerProcessing = t4.Sub(t3)
		response.ContentTransfer = t7.Sub(t4)
		response.NameLookup = t1.Sub(t0)
		response.Connect = t2.Sub(t0)
		response.StartTransfer = t4.Sub(t0)
		response.Total = t7.Sub(t0)
	}

	if isRedirect(resp) {
		loc, err := resp.Location()
		if err != nil {
			if err == http.ErrNoLocation {
				// 30x but no Location to follow, give up.
				response.Error = "no location to follow"
				return response, nil
			}

			return nil, err
		}

		redirectsFollowed++
		if redirectsFollowed > request.RedirectsToFollow {
			response.Error = fmt.Sprintf("maximum number of redirects (%d) followed", request.RedirectsToFollow)
			return response, nil
		}

		log.Printf("redirecting to %s (%d of %d)", loc.String(), redirectsFollowed, request.RedirectsToFollow)
		request.URL = loc.String()
		return check(ctx, request, redirectsFollowed)
	}

	return response, nil
}
