package engines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type artInstituteEngine struct {
	fetcher *native.Fetcher
}

func NewArtInstituteEngine(fetcher *native.Fetcher) *artInstituteEngine {
	return &artInstituteEngine{fetcher: fetcherFor(fetcher)}
}

func (e *artInstituteEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "art_institute_chicago",
		Name:     "Art Institute of Chicago",
		Priority: 49,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *artInstituteEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":      strings.TrimSpace(request.Query),
		"page":   fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"limit":  fmt.Sprintf("%d", limit),
		"fields": "id,title,image_id,date_display,artist_display,department_title",
	}
	rawURL := endpoint("https://api.artic.edu/api/v1/artworks/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
		Data []struct {
			ID            int    `json:"id"`
			Title         string `json:"title"`
			ImageID       string `json:"image_id"`
			DateDisplay   string `json:"date_display"`
			ArtistDisplay string `json:"artist_display"`
			Department    string `json:"department_title"`
		} `json:"data"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.Title == "" || item.ID == 0 {
			continue
		}
		link := fmt.Sprintf("https://www.artic.edu/artworks/%d", item.ID)
		current := result(e.Descriptor().ID, item.Title, link, item.ArtistDisplay, len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.Address = item.Department
		if item.ImageID != "" {
			current.Metadata.MediaURL = fmt.Sprintf("https://www.artic.edu/iiif/2/%s/full/843,/0/default.jpg", url.PathEscape(item.ImageID))
			current.Metadata.ThumbnailURL = current.Metadata.MediaURL
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Pagination.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}

type clevelandArtEngine struct {
	fetcher *native.Fetcher
}

func NewClevelandArtEngine(fetcher *native.Fetcher) *clevelandArtEngine {
	return &clevelandArtEngine{fetcher: fetcherFor(fetcher)}
}

func (e *clevelandArtEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "cleveland_art",
		Name:     "Cleveland Museum of Art",
		Priority: 48,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *clevelandArtEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":     strings.TrimSpace(request.Query),
		"limit": fmt.Sprintf("%d", limit),
		"skip":  fmt.Sprintf("%d", request.Offset),
	}
	rawURL := endpoint("https://openaccess-api.clevelandart.org/api/artworks/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Info struct {
			Total int `json:"total"`
		} `json:"info"`
		Data []struct {
			ID           int    `json:"id"`
			Title        string `json:"title"`
			CreationDate string `json:"creation_date"`
			URL          string `json:"url"`
			Department   string `json:"department"`
			Creators     []struct {
				Description string `json:"description"`
			} `json:"creators"`
			Images struct {
				Web struct {
					URL string `json:"url"`
				} `json:"web"`
			} `json:"images"`
		} `json:"data"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.Title == "" || item.URL == "" {
			continue
		}
		creator := ""
		if len(item.Creators) > 0 {
			creator = item.Creators[0].Description
		}
		current := result(e.Descriptor().ID, item.Title, item.URL, creator, len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.Address = item.Department
		current.Metadata.MediaURL = item.Images.Web.URL
		current.Metadata.ThumbnailURL = item.Images.Web.URL
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Info.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}

type metMuseumEngine struct {
	fetcher *native.Fetcher
}

func NewMetMuseumEngine(fetcher *native.Fetcher) *metMuseumEngine {
	return &metMuseumEngine{fetcher: fetcherFor(fetcher)}
}

func (e *metMuseumEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "met_museum",
		Name:     "The Metropolitan Museum of Art",
		Priority: 47,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage},
			Pagination:  true,
			MaxResults:  40,
		},
	}
}

func (e *metMuseumEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 40)
	searchURL := endpoint("https://collectionapi.metmuseum.org/public/collection/v1/search", map[string]string{
		"q":         strings.TrimSpace(request.Query),
		"hasImages": "true",
	})
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, searchURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Total     int   `json:"total"`
		ObjectIDs []int `json:"objectIDs"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	start := request.Offset
	if start > len(payload.ObjectIDs) {
		start = len(payload.ObjectIDs)
	}
	end := start + limit
	if end > len(payload.ObjectIDs) {
		end = len(payload.ObjectIDs)
	}
	results := make([]search.SearchResult, 0, end-start)
	rawBytes := len(body)
	for _, objectID := range payload.ObjectIDs[start:end] {
		objectURL := fmt.Sprintf("https://collectionapi.metmuseum.org/public/collection/v1/objects/%d", objectID)
		objectBody, _, objectErr := e.fetcher.Get(ctx, e.Descriptor().ID, objectURL, nil)
		if objectErr != nil {
			continue
		}
		rawBytes += len(objectBody)
		object, decodeErr := decodeJSON[struct {
			Title             string `json:"title"`
			ObjectURL         string `json:"objectURL"`
			PrimaryImage      string `json:"primaryImage"`
			PrimaryImageSmall string `json:"primaryImageSmall"`
			ArtistDisplayName string `json:"artistDisplayName"`
			ObjectDate        string `json:"objectDate"`
			Department        string `json:"department"`
		}](objectBody)
		if decodeErr != nil || object.Title == "" || object.ObjectURL == "" {
			continue
		}
		current := result(e.Descriptor().ID, object.Title, object.ObjectURL, object.ArtistDisplayName, len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.Address = object.Department
		current.Metadata.MediaURL = firstNonEmpty(object.PrimaryImage, object.PrimaryImageSmall)
		current.Metadata.ThumbnailURL = firstNonEmpty(object.PrimaryImageSmall, object.PrimaryImage)
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: end < payload.Total, HTTPStatus: status, RawBytes: rawBytes}, nil
}

type musicBrainzEngine struct {
	fetcher *native.Fetcher
}

func NewMusicBrainzEngine(fetcher *native.Fetcher) *musicBrainzEngine {
	return &musicBrainzEngine{fetcher: fetcherFor(fetcher)}
}

func (e *musicBrainzEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "musicbrainz",
		Name:     "MusicBrainz",
		Priority: 49,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindMusic},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *musicBrainzEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"query":  strings.TrimSpace(request.Query),
		"fmt":    "json",
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", request.Offset),
	}
	rawURL := endpoint("https://musicbrainz.org/ws/2/recording", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Count      int `json:"count"`
		Recordings []struct {
			ID               string `json:"id"`
			Title            string `json:"title"`
			Length           int    `json:"length"`
			FirstReleaseDate string `json:"first-release-date"`
			ArtistCredit     []struct {
				Name string `json:"name"`
			} `json:"artist-credit"`
			Releases []struct {
				Title string `json:"title"`
			} `json:"releases"`
		} `json:"recordings"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Recordings))
	for _, item := range payload.Recordings {
		if item.ID == "" || item.Title == "" {
			continue
		}
		artist := ""
		if len(item.ArtistCredit) > 0 {
			artist = item.ArtistCredit[0].Name
		}
		album := ""
		if len(item.Releases) > 0 {
			album = item.Releases[0].Title
		}
		current := result(e.Descriptor().ID, item.Title, "https://musicbrainz.org/recording/"+url.PathEscape(item.ID), artist, len(results)+1)
		current.Metadata.Type = "music"
		current.Metadata.Album = album
		current.Metadata.DurationSeconds = float64(item.Length) / 1000
		if parsed := parseTime(item.FirstReleaseDate); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Count, HTTPStatus: status, RawBytes: len(body)}, nil
}

type deezerEngine struct {
	fetcher *native.Fetcher
}

func NewDeezerEngine(fetcher *native.Fetcher) *deezerEngine {
	return &deezerEngine{fetcher: fetcherFor(fetcher)}
}

func (e *deezerEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "deezer",
		Name:     "Deezer",
		Priority: 51,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindMusic},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *deezerEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":     strings.TrimSpace(request.Query),
		"limit": fmt.Sprintf("%d", limit),
		"index": fmt.Sprintf("%d", request.Offset),
	}
	rawURL := endpoint("https://api.deezer.com/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Data []struct {
			Title    string `json:"title"`
			Link     string `json:"link"`
			Preview  string `json:"preview"`
			Duration int    `json:"duration"`
			Artist   struct {
				Name string `json:"name"`
			} `json:"artist"`
			Album struct {
				Title string `json:"title"`
			} `json:"album"`
		} `json:"data"`
		Total int `json:"total"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	if payload.Error != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_REQUEST_REJECTED, e.Descriptor().ID, false, nil)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.Title == "" || item.Link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.Link, item.Artist.Name, len(results)+1)
		current.Metadata.Type = "music"
		current.Metadata.Album = item.Album.Title
		current.Metadata.MediaURL = item.Preview
		current.Metadata.DurationSeconds = float64(item.Duration)
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}

type iTunesEngine struct {
	fetcher *native.Fetcher
}

func NewITunesEngine(fetcher *native.Fetcher) *iTunesEngine {
	return &iTunesEngine{fetcher: fetcherFor(fetcher)}
}

func (e *iTunesEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "itunes",
		Name:     "iTunes Search",
		Priority: 53,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:   []search.SearchKind{search.SearchKindMusic, search.SearchKindSoftware},
			CountryFilter: true,
			Pagination:    false,
			MaxResults:    200,
		},
	}
}

func (e *iTunesEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 200)
	values := map[string]string{
		"term":    strings.TrimSpace(request.Query),
		"media":   "music",
		"country": request.Country,
		"limit":   fmt.Sprintf("%d", limit),
		"offset":  fmt.Sprintf("%d", request.Offset),
	}
	metadataType := "music"
	if request.Kind == search.SearchKindSoftware {
		values["media"] = "software"
		metadataType = "app"
	}
	rawURL := endpoint("https://itunes.apple.com/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		ResultCount int `json:"resultCount"`
		Results     []struct {
			TrackName      string `json:"trackName"`
			CollectionName string `json:"collectionName"`
			ArtistName     string `json:"artistName"`
			TrackViewURL   string `json:"trackViewUrl"`
			ArtworkURL     string `json:"artworkUrl100"`
			ReleaseDate    string `json:"releaseDate"`
			TrackTimeMS    int    `json:"trackTimeMillis"`
		} `json:"results"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		if item.TrackName == "" || item.TrackViewURL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.TrackName, item.TrackViewURL, item.ArtistName, len(results)+1)
		current.Metadata.Type = metadataType
		current.Metadata.Album = item.CollectionName
		current.Metadata.ThumbnailURL = item.ArtworkURL
		current.Metadata.DurationSeconds = float64(item.TrackTimeMS) / 1000
		if parsed := parseTime(item.ReleaseDate); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Results) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}
