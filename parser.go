package main

import (
	"golang.org/x/net/html"
	"io"
	"net/url"
	"errors"
	"strings"
)

type Parser interface {
	// Parse HTML in order to find non-HTTPS resources.
	Parse(baseUrl string, httpBody io.Reader) (resourceUrls []string, linkUrls []string, err error)
}

// Takes a reader object and returns a slice of insecure resource urls
// found in the HTML.
// It does not close the reader. The reader should be closed from the outside.
func (f InsecureResourceFetcher) Parse(baseUrl string, httpBody io.Reader) (resourceUrls []string, linkUrls []string, err error) {

	resourceMap := make(map[string]bool)
	linkMap := make(map[string]bool)

	page := html.NewTokenizer(httpBody)
	for {
		tokenType := page.Next()
		if tokenType == html.ErrorToken {
			break
		}
		token := page.Token()

		switch {
		case tokenType == html.SelfClosingTagToken && token.DataAtom.String() == "img":
			uri, err := f.processResourceToken(token)
			if err == nil {
				resourceMap[uri] = true
			}
		case tokenType == html.StartTagToken && token.DataAtom.String() == "iframe":
			uri, err := f.processResourceToken(token)
			if err == nil {
				resourceMap[uri] = true
			}
		case tokenType == html.StartTagToken && token.DataAtom.String() == "a":
			uri, err := f.processLinkToken(token, baseUrl)
			if err == nil {
				linkMap[uri] = true
			}
		}
	}

	resourceUrls = make([]string, 0, len(resourceMap))

	for k, _ := range resourceMap {
		resourceUrls = append(resourceUrls, k)
	}

	linkUrls = make([]string, 0, len(linkMap))

	for k, _ := range linkMap {
		linkUrls = append(linkUrls, k)
	}

	return resourceUrls, linkUrls, nil
}

func (f InsecureResourceFetcher) processResourceToken(token html.Token) (string, error) {
	// Loop for tag attributes.
	for _, attr := range token.Attr {
		if attr.Key != "src" {
			continue
		}

		uri, err := url.Parse(attr.Val)
		if err != nil {
			return "", err
		}

		// Ignore relative and secure urls.
		if !uri.IsAbs() || uri.Scheme == "https" || (uri.Host != "" && strings.HasPrefix(uri.String(), "//")) {
			return "", errors.New("Uri is relative or secure. Skipped.")
		}

		return uri.String(), nil
	}

	return "", errors.New("Src has not been found. Skipped.");
}

func (f InsecureResourceFetcher) processLinkToken(token html.Token, base string) (string, error) {

	// Loop for tag attributes.
	for _, attr := range token.Attr {
		if attr.Key != "href" {
			continue
		}

		// Ignore anchors.
		if strings.HasPrefix(attr.Val, "#") {
			return "",  errors.New("Url is an anchor. Skipped.")
		}

		uri, err := url.Parse(attr.Val)
		if err != nil {
			return "", err
		}

		baseUrl, err := url.Parse(base)
		if err != nil {
			return "", err
		}

		// Return result if the uri is absolute.
		if uri.IsAbs() || (uri.Host != "" && strings.HasPrefix(uri.String(), "//")) {

			// Ignore external urls considering urls w/ WWW and w/o WWW as the same.
			if strings.TrimPrefix(uri.Host, "www.") != strings.TrimPrefix(baseUrl.Host, "www.") {
				return "", errors.New("Url is expernal. Skipped.")
			}

			return strings.TrimSuffix(uri.String(), "/"), nil
		}

		// Make it absolute if it's relative.
		absoluteUrl := f.convertToAbsolute(uri, baseUrl)

		return strings.TrimSuffix(absoluteUrl.String(), "/"), nil
	}

	return "", errors.New("Src has not been found. Skipped.");
}

func (f InsecureResourceFetcher) convertToAbsolute(href, base *url.URL) (*url.URL) {
	return base.ResolveReference(href)
}
