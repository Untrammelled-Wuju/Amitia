package management

import (
	"math"
	"testing"
)

func TestGameCenterPaginationDoesNotWrapLargePage(t *testing.T) {
	items := []int{1, 2, 3}
	if page := paginate(items, 2, 2); len(page) != 1 || page[0] != 3 {
		t.Fatalf("second page=%v", page)
	}
	for _, page := range []int{math.MaxInt, math.MaxInt/2 + 2} {
		if result := paginate(items, page, 2); len(result) != 0 {
			t.Fatalf("page=%d result=%v", page, result)
		}
	}
}
