package messengers

import (
	"github.com/wodby/walter/log"
	"io"
	"net/http"
	"time"
)

// Notifications contain credentials and pipeline output; never replay them to a redirect.
var notificationClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// postNotification bounds response consumption and avoids logging sensitive URLs or bodies.
func postNotification(req *http.Request) bool {
	resp, err := notificationClient.Do(req)
	if err != nil {
		log.Error("Notification request failed")
		return false
	}
	defer resp.Body.Close()
	const maxResponse = 64 << 10
	n, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponse+1))
	if err != nil || n > maxResponse || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Error("Notification returned an invalid response")
		return false
	}
	return true
}
