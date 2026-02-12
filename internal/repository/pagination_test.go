package repository

import (
	"testing"
)

func TestNormalizePageRequestBounds(t *testing.T) {
	tests := []struct {
		name string
		in   PageRequest
		want PageRequest
	}{
		{name: "defaults when zero", in: PageRequest{}, want: PageRequest{Page: DefaultPage, PageSize: DefaultPageSize}},
		{name: "page floored", in: PageRequest{Page: -5, PageSize: 10}, want: PageRequest{Page: DefaultPage, PageSize: 10}},
		{name: "size floored", in: PageRequest{Page: 2, PageSize: -1}, want: PageRequest{Page: 2, PageSize: DefaultPageSize}},
		{name: "size capped", in: PageRequest{Page: 2, PageSize: MaxPageSize + 50}, want: PageRequest{Page: 2, PageSize: MaxPageSize}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizePageRequest(tc.in)
			if got != tc.want {
				t.Fatalf("normalizePageRequest(%+v) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestCalcTotalPages(t *testing.T) {
	tests := []struct {
		total    int64
		pageSize int
		want     int
	}{
		{total: 0, pageSize: 10, want: 0},
		{total: 10, pageSize: 0, want: 0},
		{total: 1, pageSize: 20, want: 1},
		{total: 20, pageSize: 20, want: 1},
		{total: 21, pageSize: 20, want: 2},
	}
	for _, tc := range tests {
		got := calcTotalPages(tc.total, tc.pageSize)
		if got != tc.want {
			t.Fatalf("calcTotalPages(%d, %d) = %d, want %d", tc.total, tc.pageSize, got, tc.want)
		}
	}
}

func FuzzNormalizePageRequestInvariants(f *testing.F) {
	f.Add(0, 0)
	f.Add(-1, -1)
	f.Add(1, 1)
	f.Add(10, MaxPageSize+50)
	f.Add(9999999, 9999999)

	f.Fuzz(func(t *testing.T, page, pageSize int) {
		got := normalizePageRequest(PageRequest{Page: page, PageSize: pageSize})
		if got.Page < 1 {
			t.Fatalf("page must be >= 1, got %d", got.Page)
		}
		if got.PageSize < 1 || got.PageSize > MaxPageSize {
			t.Fatalf("page_size out of bounds: %d", got.PageSize)
		}

		again := normalizePageRequest(PageRequest{Page: page, PageSize: pageSize})
		if got != again {
			t.Fatalf("normalizePageRequest must be deterministic: first=%+v second=%+v", got, again)
		}
	})
}

func FuzzCalcTotalPagesInvariants(f *testing.F) {
	f.Add(int64(0), 10)
	f.Add(int64(10), 0)
	f.Add(int64(1), 20)
	f.Add(int64(21), 20)
	f.Add(int64(1<<62), 1)

	f.Fuzz(func(t *testing.T, total int64, pageSize int) {
		got := calcTotalPages(total, pageSize)
		if total <= 0 || pageSize <= 0 {
			if got != 0 {
				t.Fatalf("expected 0 pages for non-positive inputs, got %d (total=%d pageSize=%d)", got, total, pageSize)
			}
			return
		}

		if got < 1 {
			t.Fatalf("expected positive pages for positive inputs, got %d (total=%d pageSize=%d)", got, total, pageSize)
		}
		lowerBound := int64(got-1) * int64(pageSize)
		upperBound := int64(got) * int64(pageSize)
		if lowerBound >= total || total > upperBound {
			t.Fatalf("ceil invariant failed: pages=%d total=%d pageSize=%d bounds=(%d,%d]", got, total, pageSize, lowerBound, upperBound)
		}

		again := calcTotalPages(total, pageSize)
		if got != again {
			t.Fatalf("calcTotalPages must be deterministic: first=%d second=%d", got, again)
		}
	})
}
