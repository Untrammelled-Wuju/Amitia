package plugincontract

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

const (
	CoverageMapped   = "MAPPED"
	CoverageUnmapped = "UNMAPPED"

	HandlerKindMethod     = "method"
	HandlerKindFunction   = "function"
	HandlerKindClosure    = "closure"
	HandlerKindDelegation = "delegation"
	HandlerKindFactory    = "factory"

	ReasonHandlerUnparsed       = "route.handler-unparsed"
	ReasonHandlerMissing        = "route.handler-missing"
	ReasonRegistrationAmbiguous = "route.registration-ambiguous"
	ReasonRegistrationMissing   = "route.registration-missing"
	ReasonCallSurfaceEmpty      = "route.call-surface-empty"

	mainPackageKey = "cmd/server"
)

type RouteRef struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
}

type CoreCallRow struct {
	Method      string   `json:"method"`
	Route       string   `json:"route"`
	Handler     string   `json:"handler"`
	HandlerKind string   `json:"handlerKind"`
	Package     string   `json:"package"`
	Domain      string   `json:"domain"`
	Calls       []string `json:"calls"`
	SideEffects []string `json:"sideEffects"`
	Coverage    string   `json:"coverage"`
	Reason      string   `json:"reason,omitempty"`
}

type DomainCoverage struct {
	Domain   string `json:"domain"`
	Routes   int    `json:"routes"`
	Mapped   int    `json:"mapped"`
	Unmapped int    `json:"unmapped"`
}

type CoreCallInventory struct {
	Routes         int              `json:"routes"`
	Mapped         int              `json:"mapped"`
	Unmapped       int              `json:"unmapped"`
	Domains        []DomainCoverage `json:"domains"`
	UnmappedRoutes []CoreCallRow    `json:"unmappedRoutes"`
	Digest         string           `json:"digest"`
}

func (inventory CoreCallInventory) FullCoverage() bool {
	if inventory.Unmapped > 0 || len(inventory.Domains) == 0 {
		return false
	}
	for _, domain := range inventory.Domains {
		if domain.Mapped == 0 || domain.Unmapped > 0 {
			return false
		}
	}
	return true
}

func BuildCoreCallInventory(routes []RouteRef, sources *CoreCallSources) CoreCallInventory {
	inventory := CoreCallInventory{}
	domainIndex := map[string]int{}
	for _, route := range routes {
		row := resolveRoute(route, sources)
		inventory.Routes++
		if row.Coverage == CoverageMapped {
			inventory.Mapped++
		} else {
			inventory.Unmapped++
			inventory.UnmappedRoutes = append(inventory.UnmappedRoutes, row)
		}
		position, exists := domainIndex[row.Domain]
		if !exists {
			position = len(inventory.Domains)
			domainIndex[row.Domain] = position
			inventory.Domains = append(inventory.Domains, DomainCoverage{Domain: row.Domain})
		}
		inventory.Domains[position].Routes++
		if row.Coverage == CoverageMapped {
			inventory.Domains[position].Mapped++
		} else {
			inventory.Domains[position].Unmapped++
		}
	}
	sort.Slice(inventory.Domains, func(i, j int) bool { return inventory.Domains[i].Domain < inventory.Domains[j].Domain })
	sort.Slice(inventory.UnmappedRoutes, func(i, j int) bool {
		if inventory.UnmappedRoutes[i].Route != inventory.UnmappedRoutes[j].Route {
			return inventory.UnmappedRoutes[i].Route < inventory.UnmappedRoutes[j].Route
		}
		return inventory.UnmappedRoutes[i].Method < inventory.UnmappedRoutes[j].Method
	})
	inventory.Digest = inventory.digest(routes)
	return inventory
}

func (inventory CoreCallInventory) digest(routes []RouteRef) string {
	lines := make([]string, 0, len(inventory.Domains)+len(inventory.UnmappedRoutes)+1)
	for _, domain := range inventory.Domains {
		lines = append(lines, "domain|"+domain.Domain+"|"+itoa(domain.Routes)+"|"+itoa(domain.Mapped)+"|"+itoa(domain.Unmapped))
	}
	for _, row := range inventory.UnmappedRoutes {
		lines = append(lines, "unmapped|"+row.Method+"|"+row.Route+"|"+row.Handler+"|"+row.Reason)
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func resolveRoute(route RouteRef, sources *CoreCallSources) CoreCallRow {
	row := CoreCallRow{
		Method:  strings.ToUpper(strings.TrimSpace(route.Method)),
		Route:   strings.TrimSpace(route.Path),
		Handler: strings.TrimSpace(route.Handler),
		Domain:  domainFromRoute(route.Path),
	}
	if sources == nil {
		row.Coverage = CoverageUnmapped
		row.Reason = ReasonHandlerMissing
		return row
	}
	if packageDir, name, ok := splitInternalHandler(row.Handler); ok {
		surface, found := sources.FindFunction(packageDir, name)
		if found {
			row.Package = packageDir
			row.Domain = domainFromPackage(packageDir, row.Domain)
			row.HandlerKind = handlerKindFor(name)
			return finishRow(row, surface)
		}
		row.Package = packageDir
		row.Domain = domainFromPackage(packageDir, row.Domain)
		row.HandlerKind = handlerKindFor(name)
		if resolved, kind, ok := matchRegistration(sources, packageDir, row.Method, row.Route, name); ok {
			row.HandlerKind = kind
			return finishRow(row, resolved)
		}
		row.Coverage = CoverageUnmapped
		row.Reason = ReasonHandlerMissing
		return row
	}
	if strings.HasPrefix(row.Handler, "main.") {
		row.HandlerKind = HandlerKindClosure
		row.Package = mainPackageKey
		surface, kind := resolveMainHandler(row, sources)
		if kind != "" {
			row.HandlerKind = kind
		}
		return finishRow(row, surface)
	}
	row.Coverage = CoverageUnmapped
	row.Reason = ReasonHandlerUnparsed
	return row
}

func finishRow(row CoreCallRow, surface *FunctionSurface) CoreCallRow {
	if surface == nil {
		row.Coverage = CoverageUnmapped
		if row.Reason == "" {
			row.Reason = ReasonRegistrationMissing
		}
		return row
	}
	row.Calls = make([]string, 0, len(surface.Calls))
	effects := map[string]bool{}
	for _, call := range surface.Calls {
		row.Calls = append(row.Calls, call.Target)
		if call.SideEffect != "" {
			effects[call.SideEffect] = true
		}
	}
	row.SideEffects = make([]string, 0, len(effects))
	for effect := range effects {
		row.SideEffects = append(row.SideEffects, effect)
	}
	sort.Strings(row.SideEffects)
	if len(row.Calls) == 0 {
		row.Coverage = CoverageUnmapped
		row.Reason = ReasonCallSurfaceEmpty
		return row
	}
	row.Coverage = CoverageMapped
	return row
}

func resolveMainHandler(row CoreCallRow, sources *CoreCallSources) (*FunctionSurface, string) {
	if surface, ok := lookupMainSymbol(sources, row.Handler); ok {
		return surface, ""
	}
	if surface, kind, ok := matchRegistration(sources, mainPackageKey, row.Method, row.Route, strings.TrimPrefix(row.Handler, "main.")); ok {
		return surface, kind
	}
	candidates := make([]RouteRegistration, 0, 4)
	longest := 0
	for _, registration := range sources.Registrations {
		if registration.Package != "main" {
			continue
		}
		if !strings.EqualFold(registration.Method, row.Method) && registration.Method != "Any" {
			continue
		}
		if !pathMatches(registration.Path, row.Route) {
			continue
		}
		if registration.Handler == nil {
			continue
		}
		switch {
		case len(registration.Path) > longest:
			longest = len(registration.Path)
			candidates = candidates[:0]
			candidates = append(candidates, registration)
		case len(registration.Path) == longest:
			candidates = append(candidates, registration)
		}
	}
	if len(candidates) == 1 {
		return candidates[0].Handler, ""
	}
	if len(candidates) > 1 {
		row.Reason = ReasonRegistrationAmbiguous
		return nil, ""
	}
	row.Reason = ReasonRegistrationMissing
	return nil, ""
}

func matchRegistration(sources *CoreCallSources, packageDir, method, route, symbolName string) (*FunctionSurface, string, bool) {
	if sources == nil {
		return nil, "", false
	}
	owner, numbers := splitClosureSymbol(symbolName)
	scoped := make([]RouteRegistration, 0, 4)
	for _, registration := range sources.Registrations {
		if registration.PackageDir != packageDir {
			continue
		}
		if !strings.EqualFold(registration.Method, method) && registration.Method != "Any" {
			continue
		}
		scoped = append(scoped, registration)
	}
	tiers := []func(RouteRegistration) bool{
		func(candidate RouteRegistration) bool {
			return candidate.PathLiteral && candidate.Path != "" && pathMatches(candidate.Path, route)
		},
		func(candidate RouteRegistration) bool {
			if candidate.PathLiteral {
				return false
			}
			if candidate.Owner != owner && candidate.Owner != rootOwner(owner) {
				return false
			}
			return matchesClosureIndex(candidate, numbers)
		},
		func(candidate RouteRegistration) bool { return candidate.PathLiteral && candidate.Path == "" },
	}
	for _, tier := range tiers {
		candidates := make([]RouteRegistration, 0, len(scoped))
		for _, candidate := range scoped {
			if tier(candidate) {
				candidates = append(candidates, candidate)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		selected, found := selectRegistration(candidates, owner, numbers)
		if !found {
			return nil, "", false
		}
		return registrationSurface(sources, selected)
	}
	return nil, "", false
}

func rootOwner(owner string) string {
	if position := strings.Index(owner, "."); position >= 0 {
		return owner[:position]
	}
	return owner
}

func matchesClosureIndex(candidate RouteRegistration, numbers []int) bool {
	if candidate.Handler == nil {
		return true
	}
	if len(numbers) == 0 {
		return candidate.ClosureIndex == 0
	}
	return candidate.ClosureIndex == numbers[0]
}

func selectRegistration(candidates []RouteRegistration, owner string, numbers []int) (RouteRegistration, bool) {
	last := owner
	if position := strings.LastIndex(owner, "."); position >= 0 {
		last = owner[position+1:]
	}
	filters := []func(RouteRegistration) bool{
		func(candidate RouteRegistration) bool {
			return len(numbers) > 0 && candidate.Owner == owner && candidate.ClosureIndex == numbers[0]
		},
		func(candidate RouteRegistration) bool { return candidate.Owner == owner },
		func(candidate RouteRegistration) bool {
			if last == "" {
				return false
			}
			if candidate.Owner == last || strings.HasSuffix(candidate.Owner, "."+last) {
				return true
			}
			return strings.Contains(candidate.HandlerText, last)
		},
	}
	for _, filter := range filters {
		matched := make([]RouteRegistration, 0, len(candidates))
		for _, candidate := range candidates {
			if filter(candidate) {
				matched = append(matched, candidate)
			}
		}
		if len(matched) == 1 {
			return matched[0], true
		}
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return RouteRegistration{}, false
}

func registrationSurface(sources *CoreCallSources, registration RouteRegistration) (*FunctionSurface, string, bool) {
	if registration.Handler != nil {
		return registration.Handler, HandlerKindClosure, true
	}
	target := strings.TrimSpace(registration.HandlerText)
	if target == "" {
		return nil, "", false
	}
	if head := headIdentifier(target); head != "" {
		if surface, ok := sources.FindFunction(registration.PackageDir, head); ok {
			return surface, HandlerKindFactory, true
		}
	}
	return &FunctionSurface{
		Name:  registration.Owner,
		File:  registration.File,
		Calls: []CallSite{{Target: target, SideEffect: sideEffectFor(target)}},
	}, HandlerKindDelegation, true
}

func headIdentifier(expression string) string {
	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return ""
	}
	end := len(trimmed)
	for index, symbol := range trimmed {
		if symbol == '.' || symbol == '(' || symbol == '[' || symbol == ' ' {
			end = index
			break
		}
	}
	return trimmed[:end]
}

func splitClosureSymbol(symbolName string) (string, []int) {
	segments := strings.Split(strings.TrimSpace(symbolName), ".")
	numbers := make([]int, 0, 2)
	for len(segments) > 1 {
		last := segments[len(segments)-1]
		if !strings.HasPrefix(last, "func") {
			break
		}
		value, err := strconv.Atoi(last[4:])
		if err != nil {
			break
		}
		numbers = append([]int{value}, numbers...)
		segments = segments[:len(segments)-1]
	}
	return strings.Join(segments, "."), numbers
}

func lookupMainSymbol(sources *CoreCallSources, handler string) (*FunctionSurface, bool) {
	if sources == nil || sources.Functions == nil {
		return nil, false
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(handler), "main.")
	if !ok || rest == "" {
		return nil, false
	}
	segments := strings.Split(rest, ".")
	key := mainPackageKey + "::" + segments[0]
	if _, exists := sources.Functions[key]; !exists {
		return nil, false
	}
	for _, segment := range segments[1:] {
		if segment == segments[0] {
			continue
		}
		if !strings.HasPrefix(segment, "func") {
			return nil, false
		}
		key = key + "." + segment
	}
	surface, exists := sources.Functions[key]
	return surface, exists
}

func pathMatches(registered, route string) bool {
	registered = strings.TrimSpace(registered)
	route = strings.TrimSpace(route)
	if registered == "" || route == "" {
		return false
	}
	if registered == route {
		return true
	}
	return strings.HasSuffix(strings.TrimSuffix(route, "/"), strings.TrimSuffix(registered, "/"))
}

func splitInternalHandler(handler string) (string, string, bool) {
	const prefix = "github.com/u-ai/backend/"
	if !strings.HasPrefix(handler, prefix) {
		return "", "", false
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(handler, prefix), "-fm")
	slash := strings.LastIndex(rest, "/")
	if slash <= 0 {
		return "", "", false
	}
	dot := strings.Index(rest[slash:], ".")
	if dot <= 0 {
		return "", "", false
	}
	dot += slash
	packageDir := rest[:dot]
	name := normalizeHandlerSymbol(rest[dot+1:])
	if name == "" {
		return "", "", false
	}
	return packageDir, name, true
}

func normalizeHandlerSymbol(symbol string) string {
	replaced := strings.NewReplacer("(", "", ")", "", "*", "").Replace(symbol)
	return strings.TrimSpace(replaced)
}

func handlerKindFor(name string) string {
	if strings.Contains(name, ".") {
		return HandlerKindMethod
	}
	return HandlerKindFunction
}

func domainFromPackage(packageDir, fallback string) string {
	trimmed := strings.Trim(packageDir, "/")
	if strings.HasPrefix(trimmed, "internal/") {
		rest := strings.TrimPrefix(trimmed, "internal/")
		if index := strings.Index(rest, "/"); index >= 0 {
			return rest[:index]
		}
		if rest != "" {
			return rest
		}
	}
	if strings.HasPrefix(trimmed, "pkg/") {
		rest := strings.TrimPrefix(trimmed, "pkg/")
		if index := strings.Index(rest, "/"); index >= 0 {
			return rest[:index]
		}
		if rest != "" {
			return rest
		}
	}
	return fallback
}

func domainFromRoute(path string) string {
	trimmed := strings.TrimSpace(path)
	trimmed = strings.TrimPrefix(trimmed, "/")
	trimmed = strings.TrimPrefix(trimmed, "api/")
	trimmed = strings.TrimPrefix(trimmed, "v1/")
	if trimmed == "" {
		return "root"
	}
	if index := strings.Index(trimmed, "/"); index >= 0 {
		return trimmed[:index]
	}
	return trimmed
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
