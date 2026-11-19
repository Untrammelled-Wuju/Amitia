package main

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

const orderPlanStep int64 = 1024

type orderPlanEntry struct {
	ID    string
	Order int64
}

func planSparseOrder(entries []orderPlanEntry, movingID, beforeID string, step int64) ([]orderPlanEntry, error) {
	if movingID == "" {
		return nil, errors.New("排序对象不能为空")
	}
	if beforeID == movingID {
		return nil, fmt.Errorf("不能把 %q 移动到自身之前", movingID)
	}
	if step <= 0 {
		step = orderPlanStep
	}
	sorted := append([]orderPlanEntry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Order != sorted[j].Order {
			return sorted[i].Order < sorted[j].Order
		}
		return sorted[i].ID < sorted[j].ID
	})
	moving := orderPlanEntry{}
	remaining := make([]orderPlanEntry, 0, len(sorted))
	for _, entry := range sorted {
		if entry.ID == movingID {
			moving = entry
			continue
		}
		remaining = append(remaining, entry)
	}
	if moving.ID == "" {
		return nil, fmt.Errorf("排序对象 %q 不在当前分组内", movingID)
	}
	insertAt := len(remaining)
	if beforeID != "" {
		insertAt = -1
		for index, entry := range remaining {
			if entry.ID == beforeID {
				insertAt = index
				break
			}
		}
		if insertAt < 0 {
			return nil, fmt.Errorf("排序锚点 %q 不在当前分组内", beforeID)
		}
	}
	switch {
	case len(remaining) == 0:
		moving.Order = step
		return []orderPlanEntry{moving}, nil
	case insertAt == len(remaining):
		last := remaining[len(remaining)-1].Order
		if last <= math.MaxInt64-step {
			moving.Order = last + step
			return []orderPlanEntry{moving}, nil
		}
	case insertAt == 0:
		first := remaining[0].Order
		if first > step {
			moving.Order = first - step
			return []orderPlanEntry{moving}, nil
		}
	default:
		previous, next := remaining[insertAt-1].Order, remaining[insertAt].Order
		if next-previous > 1 {
			moving.Order = previous + (next-previous)/2
			return []orderPlanEntry{moving}, nil
		}
	}
	full := make([]orderPlanEntry, 0, len(remaining)+1)
	full = append(full, remaining[:insertAt]...)
	full = append(full, moving)
	full = append(full, remaining[insertAt:]...)
	plan := make([]orderPlanEntry, 0, len(full))
	for index := range full {
		order := int64(index+1) * step
		if full[index].Order != order {
			plan = append(plan, orderPlanEntry{ID: full[index].ID, Order: order})
		}
	}
	return plan, nil
}
