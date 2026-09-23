package engines

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

var translationPattern = regexp.MustCompile(`(?i)^(.+?)\s+(?:to|into|in)\s+([a-z][a-z-]{1,15})$`)
var currencyPattern = regexp.MustCompile(`(?i)^([0-9]+(?:\.[0-9]+)?)\s*([a-z]{3})\s+(?:to|in)\s+([a-z]{3})$`)

type myMemoryEngine struct {
	fetcher *native.Fetcher
}

func NewMyMemoryEngine(fetcher *native.Fetcher) *myMemoryEngine {
	return &myMemoryEngine{fetcher: fetcherFor(fetcher)}
}

func (e *myMemoryEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "mymemory",
		Name:     "MyMemory",
		Priority: 55,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindTranslation},
			MaxResults:  1,
		},
	}
}

func (e *myMemoryEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	text, target, ok := parseTranslationQuery(request.Query)
	if !ok {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: 200}, nil
	}
	rawURL := endpoint("https://api.mymemory.translated.net/get", map[string]string{
		"q":        text,
		"langpair": "Autodetect|" + target,
	})
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	translated := strings.TrimSpace(payload.ResponseData.TranslatedText)
	if translated == "" {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: status, RawBytes: len(body)}, nil
	}
	current := result(e.Descriptor().ID, "翻译结果", "https://mymemory.translated.net/", translated, 1)
	current.Language = target
	current.Metadata.Type = "translation"
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, []search.SearchResult{current}), HTTPStatus: status, RawBytes: len(body)}, nil
}

type dictionaryAPIEngine struct {
	fetcher *native.Fetcher
}

func NewDictionaryAPIEngine(fetcher *native.Fetcher) *dictionaryAPIEngine {
	return &dictionaryAPIEngine{fetcher: fetcherFor(fetcher)}
}

func (e *dictionaryAPIEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "dictionaryapi",
		Name:     "Free Dictionary API",
		Priority: 54,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindDictionary},
			MaxResults:  1,
		},
	}
}

func (e *dictionaryAPIEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	word := dictionaryWord(request.Query)
	if word == "" {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: 200}, nil
	}
	rawURL := "https://api.dictionaryapi.dev/api/v2/entries/en/" + url.PathEscape(word)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[[]struct {
		Word     string `json:"word"`
		Phonetic string `json:"phonetic"`
		Meanings []struct {
			PartOfSpeech string `json:"partOfSpeech"`
			Definitions  []struct {
				Definition string `json:"definition"`
				Example    string `json:"example"`
			} `json:"definitions"`
		} `json:"meanings"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	if len(payload) == 0 {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: status, RawBytes: len(body)}, nil
	}
	definitions := make([]string, 0, 4)
	for _, meaning := range payload[0].Meanings {
		for _, definition := range meaning.Definitions {
			value := strings.TrimSpace(definition.Definition)
			if meaning.PartOfSpeech != "" {
				value = meaning.PartOfSpeech + ": " + value
			}
			if definition.Example != "" {
				value += " Example: " + definition.Example
			}
			if value != "" {
				definitions = append(definitions, value)
			}
			if len(definitions) >= 4 {
				break
			}
		}
		if len(definitions) >= 4 {
			break
		}
	}
	if len(definitions) == 0 {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: status, RawBytes: len(body)}, nil
	}
	title := payload[0].Word
	if payload[0].Phonetic != "" {
		title += " " + payload[0].Phonetic
	}
	current := result(e.Descriptor().ID, title, "https://en.wiktionary.org/wiki/"+url.PathEscape(word), strings.Join(definitions, " | "), 1)
	current.Metadata.Type = "dictionary"
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, []search.SearchResult{current}), HTTPStatus: status, RawBytes: len(body)}, nil
}

type openMeteoWeatherEngine struct {
	fetcher *native.Fetcher
}

func NewOpenMeteoWeatherEngine(fetcher *native.Fetcher) *openMeteoWeatherEngine {
	return &openMeteoWeatherEngine{fetcher: fetcherFor(fetcher)}
}

func (e *openMeteoWeatherEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "open_meteo_weather",
		Name:     "Open-Meteo Weather",
		Priority: 53,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:    []search.SearchKind{search.SearchKindWeather},
			LanguageFilter: true,
			MaxResults:     1,
		},
	}
}

func (e *openMeteoWeatherEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	location := weatherLocation(request.Query)
	if location == "" {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: 200}, nil
	}
	geoBody, _, err := e.fetcher.Get(ctx, e.Descriptor().ID, endpoint("https://geocoding-api.open-meteo.com/v1/search", map[string]string{
		"name":     location,
		"count":    "1",
		"language": request.Language,
		"format":   "json",
	}), nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	geo, err := decodeJSON[struct {
		Results []struct {
			Name      string  `json:"name"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}](geoBody)
	if err != nil || len(geo.Results) == 0 {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: 200, RawBytes: len(geoBody)}, nil
	}
	place := geo.Results[0]
	forecastURL := endpoint("https://api.open-meteo.com/v1/forecast", map[string]string{
		"latitude":  strconv.FormatFloat(place.Latitude, 'f', -1, 64),
		"longitude": strconv.FormatFloat(place.Longitude, 'f', -1, 64),
		"current":   "temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m",
		"timezone":  "auto",
	})
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, forecastURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Current struct {
			Temperature         float64 `json:"temperature_2m"`
			RelativeHumidity    float64 `json:"relative_humidity_2m"`
			ApparentTemperature float64 `json:"apparent_temperature"`
			WeatherCode         int     `json:"weather_code"`
			WindSpeed           float64 `json:"wind_speed_10m"`
		} `json:"current"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	title := fmt.Sprintf("%s天气", place.Name)
	snippet := fmt.Sprintf("温度 %.1f°C，体感 %.1f°C，湿度 %.0f%%，风速 %.1f km/h，天气代码 %d", payload.Current.Temperature, payload.Current.ApparentTemperature, payload.Current.RelativeHumidity, payload.Current.WindSpeed, payload.Current.WeatherCode)
	current := result(e.Descriptor().ID, title, "https://open-meteo.com/", snippet, 1)
	current.Language = request.Language
	current.Metadata.Type = "weather"
	current.Metadata.Address = strings.TrimSpace(place.Name + ", " + place.Country)
	current.Metadata.Latitude = &place.Latitude
	current.Metadata.Longitude = &place.Longitude
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, []search.SearchResult{current}), HTTPStatus: status, RawBytes: len(body)}, nil
}

type frankfurterEngine struct {
	fetcher *native.Fetcher
}

func NewFrankfurterEngine(fetcher *native.Fetcher) *frankfurterEngine {
	return &frankfurterEngine{fetcher: fetcherFor(fetcher)}
}

func (e *frankfurterEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "frankfurter",
		Name:     "Frankfurter",
		Priority: 52,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCurrency},
			MaxResults:  1,
		},
	}
}

func (e *frankfurterEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	match := currencyPattern.FindStringSubmatch(strings.TrimSpace(request.Query))
	if len(match) != 4 {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: 200}, nil
	}
	amount, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: 200}, nil
	}
	from := strings.ToUpper(match[2])
	to := strings.ToUpper(match[3])
	rawURL := endpoint("https://api.frankfurter.app/latest", map[string]string{"from": from, "to": to})
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Date   string             `json:"date"`
		Amount float64            `json:"amount"`
		Base   string             `json:"base"`
		Rates  map[string]float64 `json:"rates"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	rate := payload.Rates[to]
	if rate <= 0 {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: status, RawBytes: len(body)}, nil
	}
	converted := amount * rate
	snippet := fmt.Sprintf("%.2f %s = %.2f %s（参考汇率 %.4f，日期 %s）", amount, from, converted, to, rate, payload.Date)
	current := result(e.Descriptor().ID, fmt.Sprintf("%s → %s", from, to), "https://www.frankfurter.app/", snippet, 1)
	current.Metadata.Type = "currency"
	current.Metadata.Currency = to
	current.Metadata.Price = &converted
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, []search.SearchResult{current}), HTTPStatus: status, RawBytes: len(body)}, nil
}

func parseTranslationQuery(query string) (string, string, bool) {
	match := translationPattern.FindStringSubmatch(strings.TrimSpace(query))
	if len(match) != 3 {
		return "", "", false
	}
	target := translateLanguageCode(match[2])
	if target == "" {
		return "", "", false
	}
	return strings.TrimSpace(match[1]), target, true
}

func translateLanguageCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	codes := map[string]string{
		"chinese": "zh-CN", "mandarin": "zh-CN", "english": "en", "japanese": "ja",
		"korean": "ko", "french": "fr", "german": "de", "spanish": "es",
		"russian": "ru", "italian": "it", "portuguese": "pt", "arabic": "ar",
	}
	if code, ok := codes[value]; ok {
		return code
	}
	if len(value) <= 8 && strings.Contains(value, "-") {
		return value
	}
	return ""
}

func dictionaryWord(query string) string {
	fields := strings.Fields(strings.TrimSpace(query))
	if len(fields) == 0 {
		return ""
	}
	if strings.EqualFold(fields[0], "define") || strings.EqualFold(fields[0], "definition") {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], " \t\r\n.,!?;:'\"()[]{}")
}

func weatherLocation(query string) string {
	value := strings.TrimSpace(query)
	lower := strings.ToLower(value)
	for _, prefix := range []string{"weather in ", "weather for ", "forecast for ", "forecast in ", "天气 "} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(value[len(prefix):])
		}
	}
	return value
}
