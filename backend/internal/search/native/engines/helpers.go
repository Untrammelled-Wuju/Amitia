package engines

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
	"golang.org/x/net/html"
)

const userAgent = "AmitiaSearch/1.0"

func endpoint(base string, values map[string]string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := parsed.Query()
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			query.Set(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func decodeJSON[T any](body []byte) (T, error) {
	var result T
	err := json.Unmarshal(body, &result)
	return result, err
}

func parseHTML(body []byte) (*html.Node, error) {
	return html.Parse(bytes.NewReader(body))
}

func findAll(node *html.Node, predicate func(*html.Node) bool) []*html.Node {
	if node == nil {
		return nil
	}
	result := make([]*html.Node, 0)
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.ElementNode && predicate(current) {
			result = append(result, current)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return result
}

func findFirst(node *html.Node, predicate func(*html.Node) bool) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && predicate(node) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirst(child, predicate); found != nil {
			return found
		}
	}
	return nil
}

func hasClass(node *html.Node, class string) bool {
	if node == nil || node.Type != html.ElementNode {
		return false
	}
	for _, item := range strings.Fields(attribute(node, "class")) {
		if item == class {
			return true
		}
	}
	return false
}

func attribute(node *html.Node, key string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func nodeText(node *html.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
			builder.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

func directText(node *html.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			builder.WriteString(child.Data)
			builder.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func firstByClass(node *html.Node, class string) *html.Node {
	return findFirst(node, func(candidate *html.Node) bool {
		return hasClass(candidate, class)
	})
}

func allByClass(node *html.Node, class string) []*html.Node {
	return findAll(node, func(candidate *html.Node) bool {
		return hasClass(candidate, class)
	})
}

func result(engineID, title, rawURL, snippet string, rank int) search.SearchResult {
	return search.SearchResult{
		Rank:    rank,
		Title:   strings.TrimSpace(title),
		URL:     strings.TrimSpace(rawURL),
		Snippet: strings.TrimSpace(snippet),
		Source: search.SearchSourceMetadata{
			Provider:     engineID,
			ProviderRank: rank,
			OriginalURL:  strings.TrimSpace(rawURL),
		},
	}
}

func response(results []search.SearchResult, hasMore bool) search.ProviderSearchResponse {
	return search.ProviderSearchResponse{
		Results:    results,
		HasMore:    hasMore,
		HTTPStatus: 200,
	}
}

func pageNumber(offset, limit int) int {
	if offset <= 0 || limit <= 0 {
		return 1
	}
	return offset/limit + 1
}

func boundedLimit(limit, fallback, maximum int) int {
	if limit <= 0 {
		limit = fallback
	}
	if maximum > 0 && limit > maximum {
		limit = maximum
	}
	return limit
}

func parsePositiveInt(value string) int {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || number < 0 {
		return 0
	}
	return number
}

func withProviders(engineID string, results []search.SearchResult) []search.SearchResult {
	for index := range results {
		results[index].Rank = index + 1
		results[index].Source.Provider = engineID
		results[index].Source.ProviderRank = index + 1
		results[index].Source.OriginalURL = results[index].URL
	}
	return results
}

func fetcherFor(fetcher *native.Fetcher) *native.Fetcher {
	if fetcher == nil {
		return native.NewFetcher(2 * 1024 * 1024)
	}
	return fetcher
}
