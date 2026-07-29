package sub

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

func TestDoFetchSubscriptionLinks_RejectsOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", subscriptionMaxBytes+1)))
	}))
	defer srv.Close()
	previousClient := subscriptionHTTPClient
	subscriptionHTTPClient = srv.Client()
	t.Cleanup(func() { subscriptionHTTPClient = previousClient })

	links, err := doFetchSubscriptionLinks(srv.URL)
	if !errors.Is(err, errSubscriptionBodyTooLarge) {
		t.Fatalf("err = %v, want errSubscriptionBodyTooLarge", err)
	}
	if links != nil {
		t.Fatalf("links = %v, want nil", links)
	}
}

func TestDoFetchSubscriptionLinks_AcceptsBodyAtLimit(t *testing.T) {
	link := "vless://example"
	body := link + "\n" + strings.Repeat("#", subscriptionMaxBytes-len(link)-1)
	if len(body) != subscriptionMaxBytes {
		t.Fatalf("fixture size = %d, want %d", len(body), subscriptionMaxBytes)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	previousClient := subscriptionHTTPClient
	subscriptionHTTPClient = srv.Client()
	t.Cleanup(func() { subscriptionHTTPClient = previousClient })

	links, err := doFetchSubscriptionLinks(srv.URL)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(links) != 1 || links[0] != link {
		t.Fatalf("links = %v, want [%q]", links, link)
	}
}

func TestDoFetchSubscriptionLinks_BlocksPrivateTargets(t *testing.T) {
	previousClient := subscriptionHTTPClient
	subscriptionHTTPClient = &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: netsafe.SSRFGuardedDialContext,
		},
	}
	t.Cleanup(func() { subscriptionHTTPClient = previousClient })

	links, err := doFetchSubscriptionLinks("http://127.0.0.1:8080/sub")
	if err == nil || !strings.Contains(err.Error(), "blocked private/internal address") {
		t.Fatalf("private subscription target was not blocked: links=%v err=%v", links, err)
	}
}
