package engines

import (
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/search/native"
)

func DefaultRegistry(fetcher *native.Fetcher) *native.Registry {
	return native.NewRegistry(
		NewDuckDuckGoEngine(fetcher),
		NewGoogleCSEEngine(fetcher),
		NewGoogleBooksEngine(fetcher),
		NewSerperEngine(fetcher),
		NewGitHubEngine(fetcher),
		NewarXivEngine(fetcher),
		NewEuropePMCBatchEngine(fetcher),
		NewPLOSEngine(fetcher),
		NewCrossrefEngine(fetcher),
		NewDataCiteEngine(fetcher),
		NewOpenAlexEngine(fetcher),
		NewOpenAIREEngine(fetcher),
		NewZenodoEngine(fetcher),
		NewSemanticScholarEngine(fetcher),
		NewPubMedEngine(fetcher),
		NewHALEngine(fetcher),
		NewOpenLibraryEngine(fetcher),
		NewPDBEEngine(fetcher),
		NewOSFEngine(fetcher),
		NewFigshareEngine(fetcher),
		NewDOAJEngine(fetcher),
		NewSpaceflightNewsEngine(fetcher),
		NewGDELTEngine(fetcher),
		NewWikipediaEngine(fetcher),
		NewWikidataEngine(fetcher),
		NewWiktionaryEngine(fetcher),
		NewWikinewsEngine(fetcher),
		NewArchWikiEngine(fetcher),
		NewGentooWikiEngine(fetcher),
		NewHackerNewsEngine(fetcher),
		NewStackExchangeEngine(fetcher),
		NewGitLabEngine(fetcher),
		NewNPMEngine(fetcher),
		NewCratesEngine(fetcher),
		NewMavenCentralEngine(fetcher),
		NewDockerHubEngine(fetcher),
		NewHuggingFaceEngine(fetcher),
		NewNuGetEngine(fetcher),
		NewPackagistEngine(fetcher),
		NewRubyGemsEngine(fetcher),
		NewCodebergEngine(fetcher),
		NewBitbucketEngine(fetcher),
		NewSoftwareHeritageEngine(fetcher),
		NewRedditEngine(fetcher),
		NewLemmyEngine(fetcher),
		NewSearchcodeEngine(fetcher),
		NewMDNEngine(fetcher),
		NewHoogleEngine(fetcher),
		NewPyPIEngine(fetcher),
		NewNASAMediaEngine(fetcher),
		NewPeerTubeEngine(fetcher),
		NewYouTubeEngine(fetcher),
		NewVimeoEngine(fetcher),
		NewWikimediaCommonsEngine(fetcher),
		NewOpenverseEngine(fetcher),
		New500pxEngine(fetcher),
		NewPexelsEngine(fetcher),
		NewUnsplashEngine(fetcher),
		NewPinterestEngine(fetcher),
		NewArtInstituteEngine(fetcher),
		NewClevelandArtEngine(fetcher),
		NewMetMuseumEngine(fetcher),
		NewRadioBrowserEngine(fetcher),
		NewGeniusEngine(fetcher),
		NewSoundCloudEngine(fetcher),
		NewMixcloudEngine(fetcher),
		NewMusicBrainzEngine(fetcher),
		NewDeezerEngine(fetcher),
		NewITunesEngine(fetcher),
		NewDailymotionEngine(fetcher),
		NewPhotonEngine(fetcher),
		NewOpenMeteoGeocodingEngine(fetcher),
		NewNominatimEngine(fetcher),
		NewOpenFoodFactsEngine(fetcher),
		NewOpenBeautyFactsEngine(fetcher),
		NewOpenProductsFactsEngine(fetcher),
		NewEbayBrowseEngine(fetcher),
		NewMyMemoryEngine(fetcher),
		NewDictionaryAPIEngine(fetcher),
		NewOpenMeteoWeatherEngine(fetcher),
		NewWttrEngine(fetcher),
		NewFrankfurterEngine(fetcher),
		NewWordnikEngine(fetcher),
		NewInternetArchiveEngine(fetcher),
	)
}

// KnownEngineIDs returns the canonical built-in native engine identifiers. It
// is intentionally derived from the same registry used at runtime so config
// validation cannot drift from the actual engine set.
func KnownEngineIDs() []string {
	registry := DefaultRegistry(nil)
	items := registry.All()
	ids := make([]string, 0, len(items))
	for _, engine := range items {
		id := strings.ToLower(strings.TrimSpace(engine.Descriptor().ID))
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func IsKnownEngineID(engineID string) bool {
	engineID = strings.ToLower(strings.TrimSpace(engineID))
	if engineID == "" {
		return false
	}
	ids := KnownEngineIDs()
	index := sort.SearchStrings(ids, engineID)
	return index < len(ids) && ids[index] == engineID
}
