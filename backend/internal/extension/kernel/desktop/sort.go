package desktop

import (
	"sort"
)

type SortContext struct {
	HostReservedIDs map[string]bool
	UserPinnedOrder map[string]int
}

func sortContributions(contributions []ResolvedDesktopContribution, ctx SortContext) []ResolvedDesktopContribution {
	result := make([]ResolvedDesktopContribution, len(contributions))
	copy(result, contributions)
	sort.SliceStable(result, func(i, j int) bool {
		a := result[i].Definition
		b := result[j].Definition
		aReserved := ctx.HostReservedIDs[a.ContributionID]
		bReserved := ctx.HostReservedIDs[b.ContributionID]
		if aReserved != bReserved {
			return aReserved
		}
		aPinned, aHas := ctx.UserPinnedOrder[a.ContributionID]
		bPinned, bHas := ctx.UserPinnedOrder[b.ContributionID]
		if aHas != bHas {
			return aHas
		}
		if aHas && bHas {
			return aPinned < bPinned
		}
		if a.Order.Group != b.Order.Group {
			return a.Order.Group < b.Order.Group
		}
		if a.Order.Priority != b.Order.Priority {
			return a.Order.Priority > b.Order.Priority
		}
		if a.Order.Before != "" && a.Order.Before == b.ContributionID {
			return true
		}
		if a.Order.After != "" && a.Order.After == b.ContributionID {
			return false
		}
		if b.Order.Before != "" && b.Order.Before == a.ContributionID {
			return false
		}
		if b.Order.After != "" && b.Order.After == a.ContributionID {
			return true
		}
		if a.ExtensionID != b.ExtensionID {
			return a.ExtensionID < b.ExtensionID
		}
		return a.ContributionID < b.ContributionID
	})
	return result
}

func SortMenuItems(contributions []ResolvedDesktopContribution, target string, ctx SortContext) []ResolvedDesktopContribution {
	var targetItems []ResolvedDesktopContribution
	for _, c := range contributions {
		if c.Definition.Target == target && c.Status == ContributionStatusRegistered {
			targetItems = append(targetItems, c)
		}
	}
	return sortContributions(targetItems, ctx)
}

func SortTrayItems(contributions []ResolvedDesktopContribution, target string, ctx SortContext) []ResolvedDesktopContribution {
	var targetItems []ResolvedDesktopContribution
	for _, c := range contributions {
		if c.Definition.Target == target && c.Status == ContributionStatusRegistered {
			targetItems = append(targetItems, c)
		}
	}
	return sortContributions(targetItems, ctx)
}

func SortShortcuts(contributions []ResolvedDesktopContribution, ctx SortContext) []ResolvedDesktopContribution {
	var shortcuts []ResolvedDesktopContribution
	for _, c := range contributions {
		if c.Definition.Shortcut != nil && c.Status == ContributionStatusRegistered {
			shortcuts = append(shortcuts, c)
		}
	}
	return sortContributions(shortcuts, ctx)
}

func insertSeparators(items []ResolvedDesktopContribution) []ResolvedDesktopContribution {
	if len(items) == 0 {
		return items
	}
	var result []ResolvedDesktopContribution
	var lastGroup string
	for _, item := range items {
		if lastGroup != "" && item.Definition.Order.Group != lastGroup {
			sep := ResolvedDesktopContribution{
				Definition: DesktopContributionDefinition{
					ContributionID: "__separator__",
					DesktopType:    DesktopTypeMenuItem,
					Order:          DesktopOrderDefinition{Group: item.Definition.Order.Group},
				},
				Status: ContributionStatusRegistered,
			}
			result = append(result, sep)
		}
		result = append(result, item)
		lastGroup = item.Definition.Order.Group
	}
	return result
}
