package util

func PageBounds(total, page, size int) (int, int) {
	pageIndex := max(page, 1) - 1
	if size <= 0 || pageIndex > total/size {
		return total, total
	}
	start := pageIndex * size
	return start, start + min(size, total-start)
}
