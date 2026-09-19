package pager_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/julienbreux/agy-sync/internal/pager"
)

func TestPaginator_PageCalculations(t *testing.T) {
	tests := []struct {
		name       string
		total      int
		pageSize   int
		wantPages  int
		page       int
		wantStart  int
		wantEnd    int
	}{
		{
			name:      "empty list",
			total:     0,
			pageSize:  10,
			wantPages: 1,
			page:      0,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "exact multiple",
			total:     30,
			pageSize:  10,
			wantPages: 3,
			page:      1,
			wantStart: 10,
			wantEnd:   20,
		},
		{
			name:      "partial last page",
			total:     25,
			pageSize:  10,
			wantPages: 3,
			page:      2,
			wantStart: 20,
			wantEnd:   25,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := pager.NewPaginator(tc.total, tc.pageSize)
			assert.Equal(t, tc.wantPages, p.TotalPages())

			p.SetPage(tc.page)
			start, end := p.PageBounds()
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantEnd, end)
		})
	}
}

func TestPaginator_Navigation(t *testing.T) {
	p := pager.NewPaginator(25, 10) // 3 pages: 0, 1, 2
	assert.Equal(t, 0, p.CurrentPage())

	p.NextPage()
	assert.Equal(t, 1, p.CurrentPage())

	p.NextPage()
	assert.Equal(t, 2, p.CurrentPage())

	// Clamp at last page
	p.NextPage()
	assert.Equal(t, 2, p.CurrentPage())

	p.PrevPage()
	assert.Equal(t, 1, p.CurrentPage())

	p.FirstPage()
	assert.Equal(t, 0, p.CurrentPage())

	// Clamp at first page
	p.PrevPage()
	assert.Equal(t, 0, p.CurrentPage())

	p.LastPage()
	assert.Equal(t, 2, p.CurrentPage())
}

func TestPaginator_RowNavigation(t *testing.T) {
	p := pager.NewPaginator(25, 10)
	assert.Equal(t, 0, p.SelectedRow())

	p.NextRow()
	assert.Equal(t, 1, p.SelectedRow())

	p.PrevRow()
	assert.Equal(t, 0, p.SelectedRow())

	// Clamp at 0
	p.PrevRow()
	assert.Equal(t, 0, p.SelectedRow())

	// Jump across pages with NextRow
	for range 15 {
		p.NextRow()
	}
	assert.Equal(t, 15, p.SelectedRow())
	assert.Equal(t, 1, p.CurrentPage(), "CurrentPage should track SelectedRow")

	// Clamp at total-1
	for range 20 {
		p.NextRow()
	}
	assert.Equal(t, 24, p.SelectedRow())
	assert.Equal(t, 2, p.CurrentPage())
}

func TestDerivePageSize(t *testing.T) {
	// Standard terminal: 24 lines, 8 lines header/footer -> 16 lines page size
	assert.Equal(t, 16, pager.DerivePageSize(24, 8, 20))

	// Large terminal: 60 lines, 10 lines header/footer -> 50 lines
	assert.Equal(t, 50, pager.DerivePageSize(60, 10, 20))

	// Small terminal: 6 lines, 8 overhead -> fallback to minimum 1
	assert.Equal(t, 1, pager.DerivePageSize(6, 8, 20))

	// Non-positive terminal height -> fallback to default
	assert.Equal(t, 20, pager.DerivePageSize(0, 8, 20))
	assert.Equal(t, 20, pager.DerivePageSize(-1, 8, 20))
}

func TestParseKey(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected pager.KeyAction
	}{
		{"q to quit", []byte{'q'}, pager.ActionQuit},
		{"Q to quit", []byte{'Q'}, pager.ActionQuit},
		{"Esc to quit", []byte{0x1b}, pager.ActionQuit},
		{"Ctrl+C to quit", []byte{0x03}, pager.ActionQuit},
		{"Up arrow", []byte{0x1b, '[', 'A'}, pager.ActionPrevRow},
		{"Down arrow", []byte{0x1b, '[', 'B'}, pager.ActionNextRow},
		{"Right arrow", []byte{0x1b, '[', 'C'}, pager.ActionNextPage},
		{"Left arrow", []byte{0x1b, '[', 'D'}, pager.ActionPrevPage},
		{"k vim up", []byte{'k'}, pager.ActionPrevRow},
		{"j vim down", []byte{'j'}, pager.ActionNextRow},
		{"h vim left", []byte{'h'}, pager.ActionPrevPage},
		{"l vim right", []byte{'l'}, pager.ActionNextPage},
		{"Space next page", []byte{' '}, pager.ActionNextPage},
		{"Page Up", []byte{0x1b, '[', '5', '~'}, pager.ActionPrevPage},
		{"Page Down", []byte{0x1b, '[', '6', '~'}, pager.ActionNextPage},
		{"Home", []byte{0x1b, '[', 'H'}, pager.ActionFirstPage},
		{"End", []byte{0x1b, '[', 'F'}, pager.ActionLastPage},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action, n := pager.ParseKey(tc.input)
			assert.Equal(t, tc.expected, action)
			assert.Equal(t, len(tc.input), n)
		})
	}
}

func TestPager_NonTTYStream(t *testing.T) {
	items := []string{"conv-1", "conv-2", "conv-3"}
	var out bytes.Buffer

	p := pager.New(pager.Options{
		IsTTY:  false,
		Out:    &out,
		In:     strings.NewReader(""),
		Height: 24,
	})

	err := p.Run(len(items), func(start, end, selected int, out *bytes.Buffer) {
		for i := start; i < end; i++ {
			out.WriteString(items[i] + "\n")
		}
	})
	require.NoError(t, err)
	output := out.String()
	assert.Contains(t, output, "conv-1")
	assert.Contains(t, output, "conv-2")
	assert.Contains(t, output, "conv-3")
}

func TestPager_InteractiveSimulation(t *testing.T) {
	items := make([]string, 25)
	for i := range 25 {
		items[i] = "item-" + string(rune('A'+i))
	}

	// Sequence: Down arrow (j), Right arrow (l), and 'q' to quit
	input := []byte{'j', 'l', 'q'}
	var out bytes.Buffer

	renderCalls := 0
	lastStart := -1

	p := pager.New(pager.Options{
		IsTTY:           true,
		Out:             &out,
		In:              bytes.NewReader(input),
		Height:          14, // height 14, overhead 4 -> pageSize 10
		Overhead:        4,
		DisableRawMode:  true, // for hermetic testing without OS terminal fd
		AlternateScreen: false,
	})

	err := p.Run(len(items), func(start, end, selected int, out *bytes.Buffer) {
		renderCalls++
		lastStart = start
		for i := start; i < end; i++ {
			out.WriteString(items[i] + "\n")
		}
	})
	require.NoError(t, err)
	// Initial render + after 'j' + after 'l' = at least 3 renders
	assert.GreaterOrEqual(t, renderCalls, 3)
	// After 'l' (Right arrow / next page), lastStart should be page 1 start index = 10
	assert.Equal(t, 10, lastStart)
}
