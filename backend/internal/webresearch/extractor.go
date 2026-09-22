package webresearch

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

type extractedDocument struct {
	Title   string
	Content string
	Links   []Link
	Blocks  []string
	Dynamic bool
}

type scoredBlock struct {
	Index int
	Text  string
	Score float64
}

func extractHTML(raw string, baseURL string, query string, maxChars int) (extractedDocument, error) {
	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return extractedDocument{}, err
	}
	base, _ := url.Parse(baseURL)
	title := findTitle(root)
	blocks := make([]string, 0, 128)
	links := make([]Link, 0, 64)
	linkSeen := map[string]struct{}{}
	collectLinks(root, base, &links, linkSeen, false)
	walkHTML(root, base, &blocks, false)
	blocks = normalizeBlocks(blocks)
	selected := selectBlocks(blocks, query, maxChars)
	content := strings.Join(selected, "\n\n")
	if maxChars > 0 && len([]rune(content)) > maxChars {
		content = string([]rune(content)[:maxChars])
	}
	dynamic := len([]rune(content)) < 240 && (strings.Contains(strings.ToLower(raw), "<script") || strings.Contains(strings.ToLower(raw), "id=\"root\"") || strings.Contains(strings.ToLower(raw), "id=\"app\""))
	return extractedDocument{Title: title, Content: content, Links: links, Blocks: selected, Dynamic: dynamic}, nil
}

func findTitle(root *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if title != "" {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "title") {
			title = normalizeSpace(nodeText(node))
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return title
}

func walkHTML(node *html.Node, base *url.URL, blocks *[]string, skipped bool) {
	if node == nil {
		return
	}
	nowSkipped := skipped || hiddenHTMLNode(node)
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if !nowSkipped && isBlockTag(tag) {
			text := normalizeSpace(nodeTextFiltered(node))
			if len([]rune(text)) >= 20 {
				prefix := ""
				switch tag {
				case "h1":
					prefix = "# "
				case "h2":
					prefix = "## "
				case "h3":
					prefix = "### "
				case "h4", "h5", "h6":
					prefix = "#### "
				case "li":
					prefix = "- "
				}
				*blocks = append(*blocks, prefix+text)
				return
			}
		}
	}
	if nowSkipped {
		return
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, base, blocks, nowSkipped)
	}
}

func collectLinks(node *html.Node, base *url.URL, links *[]Link, seen map[string]struct{}, skipped bool) {
	if node == nil {
		return
	}
	nowSkipped := skipped || hiddenHTMLNode(node)
	if nowSkipped {
		return
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "a") {
		var href string
		for _, attr := range node.Attr {
			if strings.EqualFold(attr.Key, "href") {
				href = strings.TrimSpace(attr.Val)
				break
			}
		}
		if resolved := resolveLink(base, href); resolved != "" {
			if _, exists := seen[resolved]; !exists {
				seen[resolved] = struct{}{}
				*links = append(*links, Link{ID: len(*links) + 1, Text: truncateRunes(normalizeSpace(nodeTextFiltered(node)), 256), URL: resolved})
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectLinks(child, base, links, seen, nowSkipped)
	}
}

func hiddenHTMLNode(node *html.Node) bool {
	if node == nil || node.Type != html.ElementNode {
		return false
	}
	tag := strings.ToLower(node.Data)
	switch tag {
	case "script", "style", "svg", "noscript", "nav", "footer", "header", "aside", "form", "template":
		return true
	}
	for _, attr := range node.Attr {
		key := strings.ToLower(attr.Key)
		value := strings.ToLower(strings.TrimSpace(attr.Val))
		if key == "hidden" || (key == "aria-hidden" && value == "true") {
			return true
		}
		if key == "style" && (strings.Contains(value, "display:none") || strings.Contains(value, "display: none") || strings.Contains(value, "visibility:hidden") || strings.Contains(value, "visibility: hidden")) {
			return true
		}
	}
	return false
}

func isBlockTag(tag string) bool {
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6", "p", "li", "pre", "blockquote", "td", "th":
		return true
	default:
		return false
	}
}

func nodeText(node *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
	return b.String()
}

func nodeTextFiltered(node *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if hiddenHTMLNode(n) {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
	return b.String()
}

func resolveLink(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(strings.ToLower(href), "javascript:") || strings.HasPrefix(strings.ToLower(href), "data:") || strings.HasPrefix(strings.ToLower(href), "mailto:") || strings.HasPrefix(strings.ToLower(href), "tel:") {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

func normalizeBlocks(blocks []string) []string {
	out := make([]string, 0, len(blocks))
	seen := map[string]struct{}{}
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		key := strings.ToLower(block)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, block)
	}
	return out
}

func selectBlocks(blocks []string, query string, maxChars int) []string {
	if len(blocks) == 0 {
		return nil
	}
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return takeBlocks(blocks, maxChars)
	}
	df := map[string]int{}
	blockTokens := make([][]string, len(blocks))
	for i, block := range blocks {
		tokens := tokenize(block)
		blockTokens[i] = tokens
		unique := map[string]struct{}{}
		for _, token := range tokens {
			unique[token] = struct{}{}
		}
		for token := range unique {
			df[token]++
		}
	}
	scored := make([]scoredBlock, 0, len(blocks))
	n := float64(len(blocks))
	for i, tokens := range blockTokens {
		if len(tokens) == 0 {
			continue
		}
		tf := map[string]int{}
		for _, token := range tokens {
			tf[token]++
		}
		score := 0.0
		for _, token := range queryTokens {
			count := tf[token]
			if count == 0 {
				continue
			}
			idf := math.Log(1 + (n-float64(df[token])+0.5)/(float64(df[token])+0.5))
			score += (float64(count) / (float64(count) + 1.2)) * idf
		}
		if score > 0 {
			scored = append(scored, scoredBlock{Index: i, Text: blocks[i], Score: score})
		}
	}
	if len(scored) == 0 {
		return takeBlocks(blocks, maxChars)
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Index < scored[j].Index
	})
	selectedIndexes := map[int]struct{}{}
	chars := 0
	for _, item := range scored {
		if maxChars > 0 && chars >= maxChars {
			break
		}
		selectedIndexes[item.Index] = struct{}{}
		chars += len([]rune(item.Text))
		if item.Index > 0 && chars < maxChars {
			selectedIndexes[item.Index-1] = struct{}{}
			chars += len([]rune(blocks[item.Index-1]))
		}
		if item.Index+1 < len(blocks) && chars < maxChars {
			selectedIndexes[item.Index+1] = struct{}{}
			chars += len([]rune(blocks[item.Index+1]))
		}
	}
	indexes := make([]int, 0, len(selectedIndexes))
	for index := range selectedIndexes {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	out := make([]string, 0, len(indexes))
	chars = 0
	for _, index := range indexes {
		block := blocks[index]
		if maxChars > 0 && chars+len([]rune(block)) > maxChars && len(out) > 0 {
			break
		}
		out = append(out, block)
		chars += len([]rune(block))
	}
	return out
}

func takeBlocks(blocks []string, maxChars int) []string {
	if maxChars <= 0 {
		return append([]string(nil), blocks...)
	}
	out := make([]string, 0, len(blocks))
	chars := 0
	for _, block := range blocks {
		blockChars := len([]rune(block))
		if chars+blockChars > maxChars && len(out) > 0 {
			break
		}
		out = append(out, block)
		chars += blockChars
	}
	return out
}

func buildEvidence(conversationID, pageRefID, query string, blocks []string, maxItems, maxChars int) []Evidence {
	if maxItems <= 0 {
		maxItems = 8
	}
	queryTokens := tokenize(query)
	items := make([]Evidence, 0, minInt(maxItems, len(blocks)))
	type candidate struct {
		index int
		text  string
		score float64
	}
	candidates := make([]candidate, 0, len(blocks))
	for i, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		score := overlapScore(queryTokens, tokenize(block))
		if len(queryTokens) == 0 {
			score = 1.0 / float64(i+1)
		}
		candidates = append(candidates, candidate{index: i, text: truncateRunes(block, maxChars), score: score})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].index < candidates[j].index
	})
	now := nowUTC()
	evidenceQuery := normalizedQuery(query)
	for _, candidate := range candidates {
		if len(items) >= maxItems {
			break
		}
		textHash := hashText(candidate.text)
		items = append(items, Evidence{
			ID:             newID("ev"),
			ConversationID: conversationID,
			PageRefID:      pageRefID,
			Query:          evidenceQuery,
			Text:           candidate.text,
			TextHash:       textHash,
			Locator:        EvidenceLocator{Kind: "html_block", BlockIndex: candidate.index},
			Relevance:      candidate.score,
			CreatedAt:      now,
		})
	}
	return items
}

func tokenize(value string) []string {
	value = strings.ToLower(value)
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.In(r, unicode.Han))
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len([]rune(field)) < 2 {
			continue
		}
		out = append(out, field)
	}
	return out
}

func overlapScore(queryTokens, blockTokens []string) float64 {
	if len(queryTokens) == 0 || len(blockTokens) == 0 {
		return 0
	}
	set := map[string]int{}
	for _, token := range blockTokens {
		set[token]++
	}
	hits := 0.0
	for _, token := range queryTokens {
		if set[token] > 0 {
			hits++
		}
	}
	return hits / math.Sqrt(float64(len(queryTokens))*float64(len(blockTokens))+1)
}

func normalizeSpace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
