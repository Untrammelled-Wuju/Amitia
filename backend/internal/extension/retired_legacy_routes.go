package extension

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type retiredLegacyRoute struct {
	Method string
	Path   string
}

var retiredExtensionLegacyRoutes = []retiredLegacyRoute{
	{http.MethodPost, "/packages/install"},

	{http.MethodPost, "/packages/import/preview"},
	{http.MethodGet, "/packages/import/sessions/:sessionId"},
	{http.MethodPost, "/packages/import/install"},
	{http.MethodDelete, "/packages/import/sessions/:sessionId"},

	{http.MethodGet, "/packages/metrics"},

	{http.MethodGet, "/package-operations"},
	{http.MethodGet, "/package-operations/:operationId"},
	{http.MethodGet, "/package-operations/:operationId/final-gate"},

	{http.MethodGet, "/signers"},
	{http.MethodPost, "/signers/:fingerprint/trust"},
	{http.MethodPost, "/signers/:fingerprint/untrust"},

	{http.MethodGet, "/:id/exports/:exportId"},
	{http.MethodPost, "/:id/export"},

	{http.MethodPost, "/:id/upgrade/preview"},
	{http.MethodPost, "/:id/upgrade"},

	{http.MethodGet, "/:id/versions"},
	{http.MethodGet, "/:id/versions/compare"},

	{http.MethodPost, "/:id/versions/:version/rollback"},
	{http.MethodGet, "/:id/versions/:version/rollback/preview"},
	{http.MethodPost, "/:id/versions/:version/rollback/preview"},

	{http.MethodGet, "/:id/dependencies"},

	{http.MethodGet, "/:id/uninstall/preview"},
	{http.MethodPost, "/:id/uninstall/preview"},
	{http.MethodPost, "/:id/uninstall"},
	{http.MethodDelete, "/:id"},
}

var retiredRootLegacyRoutes = []retiredLegacyRoute{
	{
		http.MethodGet,
		"/extension-package-operations/:operationId",
	},
	{
		http.MethodGet,
		"/extension-package-operations/:operationId/final-gate",
	},
}

func registerRetiredLegacyRoutes(
	group *gin.RouterGroup,
	routes []retiredLegacyRoute,
) {
	for _, route :=
		range routes {
		group.Handle(
			route.Method,
			route.Path,
			retiredLegacyPackageEndpoint,
		)
	}
}
