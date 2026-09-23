package engines

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type openFactsEngine struct {
	fetcher  *native.Fetcher
	id       string
	name     string
	domain   string
	priority int
	weight   float64
}

func NewOpenFoodFactsEngine(fetcher *native.Fetcher) *openFactsEngine {
	return &openFactsEngine{fetcher: fetcherFor(fetcher), id: "openfoodfacts", name: "Open Food Facts", domain: "world.openfoodfacts.org", priority: 48, weight: 0.6}
}

func NewOpenBeautyFactsEngine(fetcher *native.Fetcher) *openFactsEngine {
	return &openFactsEngine{fetcher: fetcherFor(fetcher), id: "openbeautyfacts", name: "Open Beauty Facts", domain: "world.openbeautyfacts.org", priority: 44, weight: 0.5}
}

func NewOpenProductsFactsEngine(fetcher *native.Fetcher) *openFactsEngine {
	return &openFactsEngine{fetcher: fetcherFor(fetcher), id: "openproductsfacts", name: "Open Products Facts", domain: "world.openproductsfacts.org", priority: 43, weight: 0.5}
}

func (e *openFactsEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       e.id,
		Name:     e.name,
		Priority: e.priority,
		Weight:   e.weight,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindProduct},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *openFactsEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"search_terms":  strings.TrimSpace(request.Query),
		"search_simple": "1",
		"action":        "process",
		"json":          "1",
		"page_size":     strconv.Itoa(limit),
		"page":          strconv.Itoa(pageNumber(request.Offset, limit)),
		"fields":        "code,product_name,brands,nutriscore_grade,image_front_small_url,countries,url",
	}
	rawURL := endpoint("https://"+e.domain+"/cgi/search.pl", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Products []struct {
			Code      string `json:"code"`
			Name      string `json:"product_name"`
			Brands    string `json:"brands"`
			Grade     string `json:"nutriscore_grade"`
			Image     string `json:"image_front_small_url"`
			Countries string `json:"countries"`
			URL       string `json:"url"`
		} `json:"products"`
		Count int `json:"count"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Products))
	for _, item := range payload.Products {
		title := strings.TrimSpace(item.Brands + " " + item.Name)
		if title == "" {
			continue
		}
		link := item.URL
		if link == "" && item.Code != "" {
			link = "https://" + e.domain + "/product/" + url.PathEscape(item.Code)
		}
		if link == "" {
			continue
		}
		current := result(e.Descriptor().ID, title, link, item.Countries, len(results)+1)
		current.Metadata.ProductID = item.Code
		current.Metadata.Type = "product"
		current.Metadata.ThumbnailURL = item.Image
		current.Metadata.Merchant = item.Brands
		current.Metadata.Availability = item.Grade
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Count, HTTPStatus: status, RawBytes: len(body)}, nil
}
