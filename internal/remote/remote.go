// Package remote makes outbound API calls and turns platform errors into readable messages.
package remote

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Shaitrish/37studio/internal/textutil"
)

// Multipart streams some fields and one file part as a multipart/form-data POST: final videos can be hundreds of MB.
func Multipart(u string, fields map[string]string, fileField, fileName string, r io.Reader, auth string) *http.Request {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		for k, v := range fields {
			mw.WriteField(k, v)
		}
		part, err := mw.CreateFormFile(fileField, fileName)
		if err == nil {
			_, err = io.Copy(part, r)
		}
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()
	req, _ := http.NewRequest("POST", u, pr)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

// CallHeader is callWith for replies whose answer is a header: LinkedIn's upload ETags and new post ids.
func CallHeader(c *http.Client, req *http.Request) (http.Header, error) {
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", strings.TrimPrefix(req.URL.Host, "www."), Message(raw))
	}
	return res.Header, nil
}

var Client = &http.Client{Timeout: time.Minute}

var Uploads = &http.Client{} // uploads can take many minutes on a slow uplink

func Get(u, auth string) *http.Request {
	req, _ := http.NewRequest("GET", u, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

func Form(method, u string, v url.Values, auth string) *http.Request {
	req, _ := http.NewRequest(method, u, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

func JSON(method, u string, body any, auth string) *http.Request {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, u, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

func Call(req *http.Request, out any) error { return CallWith(Client, req, out) }

// CallWith does one API call and decodes the reply into out. Errors carry the platform's own message, which is what
// the editor needs to see ("video too long", "permission missing"). TikTok answers success as {"error": {"code": "ok"}}.
func CallWith(c *http.Client, req *http.Request, out any) error {
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var e struct {
		Error json.RawMessage `json:"error"`
	}
	json.Unmarshal(raw, &e)
	var code struct {
		Code string `json:"code"`
	}
	failed := res.StatusCode >= 300 || (len(e.Error) > 0 && string(e.Error) != "null" && !(json.Unmarshal(e.Error, &code) == nil && code.Code == "ok"))
	if failed {
		return fmt.Errorf("%s: %s", strings.TrimPrefix(req.URL.Host, "www."), Message(raw))
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func Message(raw []byte) string {
	var e struct {
		Error   json.RawMessage `json:"error"`
		Desc    string          `json:"error_description"`
		Message string          `json:"message"` // LinkedIn, Bluesky
		Detail  string          `json:"detail"`  // X
	}
	json.Unmarshal(raw, &e)
	var inner struct {
		Message string `json:"message"`
		UserMsg string `json:"error_user_msg"`
	}
	if json.Unmarshal(e.Error, &inner) == nil && inner.Message != "" {
		return cmp.Or(inner.UserMsg, inner.Message)
	}
	var s string
	json.Unmarshal(e.Error, &s)
	return cmp.Or(e.Desc, cmp.Or(e.Message, cmp.Or(e.Detail, cmp.Or(s, textutil.Tail(string(raw), 300)))))
}

// AllowPrivate lets pasted URLs point into this machine's own network (STUDIO37_ALLOW_PRIVATE_URLS=1), for a site
// or server on the LAN.
var AllowPrivate = os.Getenv("STUDIO37_ALLOW_PRIVATE_URLS") == "1"

// Public refuses a host that is, or resolves to, a loopback, private, link-local or unspecified address, so a pasted
// URL can't make the server fetch from its own network: cloud metadata, other containers on the Docker network.
// ponytail: checked when the URL comes in; DNS that changes its answer before the fetch gets past it. A dialer
// Control hook would close that, but the fetches run in yt-dlp and HyperFrames, not here.
func Public(host string) error {
	if AllowPrivate {
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("can't find %s; check the address", host)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("%s is on a private network, which 37Studio doesn't fetch from; to allow it, set STUDIO37_ALLOW_PRIVATE_URLS=1", host)
		}
	}
	return nil
}
