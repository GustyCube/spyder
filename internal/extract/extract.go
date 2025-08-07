package extract

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

func Apex(host string) string {
	h := strings.ToLower(host)
	if e, err := publicsuffix.EffectiveTLDPlusOne(h); err == nil { return e }
	return h
}

func ParseLinks(base *url.URL, body io.Reader) ([]string, error) {
	z := html.NewTokenizer(body)
	var out []string
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if z.Err() == io.EOF { return out, nil }
			return out, z.Err()
		}
		if tt == html.StartTagToken || tt == html.SelfClosingTagToken {
			t := z.Token()
			switch strings.ToLower(t.Data) {
			case "a", "link":
				for _, a := range t.Attr {
					if strings.EqualFold(a.Key, "href") {
						u, err := url.Parse(strings.TrimSpace(a.Val)); if err == nil {
							if u.Scheme == "" { u.Scheme = base.Scheme }
							if u.Host == "" { u.Host = base.Host }
							out = append(out, u.String())
						}
					}
				}
			case "script", "img", "iframe", "source":
				for _, a := range t.Attr {
					if strings.EqualFold(a.Key, "src") {
						u, err := url.Parse(strings.TrimSpace(a.Val)); if err == nil {
							if u.Scheme == "" { u.Scheme = base.Scheme }
							if u.Host == "" { u.Host = base.Host }
							out = append(out, u.String())
						}
					}
				}
			}
		}
	}
}

func ExternalDomains(baseHost string, urls []string) []string {
	baseApex := Apex(baseHost)
	seen := make(map[string]struct{})
	var out []string
	for _, s := range urls {
		u, err := url.Parse(s); if err != nil { continue }
		h := strings.ToLower(u.Hostname()); if h == "" { continue }
		if Apex(h) == baseApex { continue }
		if _, ok := seen[h]; ok { continue }
		seen[h] = struct{}{}; out = append(out, h)
	}
	return out
}
