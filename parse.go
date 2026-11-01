package main

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// htmlInfo is everything webmri extracts from one HTML document.
type htmlInfo struct {
	title            string
	generator        string
	description      string
	scripts          []string
	stylesheetURLs   []string
	styleBodies      []string
	links            []string
	forms            []FormInfo
	feeds            []FeedInfo
	images           int
	imagesMissingAlt int
	imagesEmptyAlt   int
	hasLang          bool
	hasH1            bool
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(c *html.Node) {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
		for ch := c.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}

// parseHTML extracts structure facts from one document. base resolves
// relative references; pageURL is recorded on forms.
func parseHTML(body []byte, base *url.URL, pageURL string) *htmlInfo {
	info := &htmlInfo{}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return info
	}
	base = documentBase(doc, base)
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "html":
				info.hasLang = hasAttr(n, "lang")
			case "h1":
				info.hasH1 = true
			case "img":
				info.images++
				if !hasAttr(n, "alt") {
					info.imagesMissingAlt++
				} else if attrVal(n, "alt") == "" {
					info.imagesEmptyAlt++
				}
			case "title":
				if info.title == "" {
					info.title = nodeText(n)
				}
			case "meta":
				name := strings.ToLower(attrVal(n, "name"))
				switch name {
				case "generator":
					if info.generator == "" {
						info.generator = strings.TrimSpace(attrVal(n, "content"))
					}
				case "description":
					if info.description == "" {
						info.description = strings.TrimSpace(attrVal(n, "content"))
					}
				}
			case "script":
				if src := cleanLink(base, attrVal(n, "src")); src != "" {
					info.scripts = append(info.scripts, src)
				}
			case "link":
				rel := strings.Fields(strings.ToLower(attrVal(n, "rel")))
				hasRel := func(want string) bool {
					for _, value := range rel {
						if value == want {
							return true
						}
					}
					return false
				}
				if hasRel("stylesheet") {
					if href := cleanLink(base, attrVal(n, "href")); href != "" {
						info.stylesheetURLs = append(info.stylesheetURLs, href)
					}
				}
				feedType := strings.ToLower(strings.TrimSpace(strings.Split(attrVal(n, "type"), ";")[0]))
				if hasRel("alternate") && (feedType == "application/rss+xml" || feedType == "application/atom+xml") {
					if href := cleanLink(base, attrVal(n, "href")); href != "" {
						info.feeds = append(info.feeds, FeedInfo{URL: href, Page: pageURL})
					}
				}
			case "style":
				if t := nodeText(n); t != "" {
					info.styleBodies = append(info.styleBodies, t)
				}
			case "a":
				if href := cleanLink(base, attrVal(n, "href")); href != "" {
					info.links = append(info.links, href)
				}
			case "form":
				info.forms = append(info.forms, parseForm(n, base, pageURL))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	info.scripts = dedupeStrings(info.scripts)
	info.stylesheetURLs = dedupeStrings(info.stylesheetURLs)
	info.feeds = dedupeFeeds(info.feeds)
	info.links = dedupeStrings(info.links)
	return info
}

// documentBase returns the base URL relative references resolve against: the
// first <base href> in the document, resolved against page, or page itself.
func documentBase(doc *html.Node, page *url.URL) *url.URL {
	var baseNode *html.Node
	var find func(n *html.Node)
	find = func(n *html.Node) {
		if baseNode != nil {
			return
		}
		if n.Type == html.ElementNode && strings.ToLower(n.Data) == "base" && hasAttr(n, "href") {
			baseNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	if baseNode == nil {
		return page
	}
	resolved, err := page.Parse(strings.TrimSpace(attrVal(baseNode, "href")))
	if err != nil {
		return page
	}
	return resolved
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return true
		}
	}
	return false
}

func parseForm(form *html.Node, base *url.URL, pageURL string) FormInfo {
	fi := FormInfo{
		Page:   pageURL,
		Action: cleanLink(base, attrVal(form, "action")),
		Method: strings.ToUpper(attrVal(form, "method")),
	}
	if fi.Method == "" {
		fi.Method = "GET"
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "input":
				name := attrVal(n, "name")
				typ := attrVal(n, "type")
				if name != "" {
					if typ != "" {
						fi.Inputs = append(fi.Inputs, name+" ("+typ+")")
					} else {
						fi.Inputs = append(fi.Inputs, name)
					}
				}
			case "select", "textarea":
				if name := attrVal(n, "name"); name != "" {
					fi.Inputs = append(fi.Inputs, name)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(form)
	fi.Inputs = dedupeStrings(fi.Inputs)
	return fi
}

// headerString renders the headers webmri's rules care about, for evidence.
func headerString(h http.Header, key string) string {
	return h.Get(key)
}
