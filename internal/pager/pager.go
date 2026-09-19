package pager

import (
	"bytes"
	"errors"
	"io"
	"os"

	"golang.org/x/term"
)

// KeyAction represents a user navigation intent in the interactive pager.
type KeyAction int

const (
	ActionNone KeyAction = iota
	ActionPrevRow
	ActionNextRow
	ActionPrevPage
	ActionNextPage
	ActionFirstPage
	ActionLastPage
	ActionQuit
)

// Paginator manages pagination indices, bounds, and cursor selection.
type Paginator struct {
	totalItems  int
	pageSize    int
	currentPage int
	selectedRow int
}

// NewPaginator constructs a new Paginator instance.
func NewPaginator(totalItems, pageSize int) *Paginator {
	if pageSize < 1 {
		pageSize = 1
	}
	return &Paginator{
		totalItems: totalItems,
		pageSize:   pageSize,
	}
}

// TotalPages returns the total count of pages available.
func (p *Paginator) TotalPages() int {
	if p.totalItems <= 0 {
		return 1
	}
	pages := p.totalItems / p.pageSize
	if p.totalItems%p.pageSize != 0 {
		pages++
	}
	return pages
}

// CurrentPage returns the 0-indexed current page.
func (p *Paginator) CurrentPage() int {
	return p.currentPage
}

// SetPage sets the current page within valid bounds.
func (p *Paginator) SetPage(page int) {
	if page < 0 {
		page = 0
	}
	maxPage := p.TotalPages() - 1
	if page > maxPage {
		page = maxPage
	}
	p.currentPage = page
}

// PageSize returns the current page size.
func (p *Paginator) PageSize() int {
	return p.pageSize
}

// SetPageSize updates page size and adjusts current page.
func (p *Paginator) SetPageSize(size int) {
	if size < 1 {
		size = 1
	}
	p.pageSize = size
	if p.selectedRow >= 0 {
		p.currentPage = p.selectedRow / p.pageSize
	}
}

// SelectedRow returns the 0-indexed selected row across all items.
func (p *Paginator) SelectedRow() int {
	return p.selectedRow
}

// SetSelectedRow sets the selected row and synchronizes the current page.
func (p *Paginator) SetSelectedRow(row int) {
	if row < 0 {
		row = 0
	}
	maxRow := max(0, p.totalItems-1)
	if row > maxRow {
		row = maxRow
	}
	p.selectedRow = row
	if p.pageSize > 0 {
		p.currentPage = row / p.pageSize
	}
}

// PageBounds returns the start and end (exclusive) indices for the current page.
func (p *Paginator) PageBounds() (int, int) {
	if p.totalItems <= 0 {
		return 0, 0
	}
	start := min(p.currentPage*p.pageSize, p.totalItems)
	end := min(start+p.pageSize, p.totalItems)
	return start, end
}

// NextPage advances to the next page.
func (p *Paginator) NextPage() {
	if p.currentPage < p.TotalPages()-1 {
		p.currentPage++
		p.selectedRow = p.currentPage * p.pageSize
	}
}

// PrevPage returns to the previous page.
func (p *Paginator) PrevPage() {
	if p.currentPage > 0 {
		p.currentPage--
		p.selectedRow = p.currentPage * p.pageSize
	}
}

// FirstPage jumps to the first page.
func (p *Paginator) FirstPage() {
	p.currentPage = 0
	p.selectedRow = 0
}

// LastPage jumps to the final page.
func (p *Paginator) LastPage() {
	p.currentPage = p.TotalPages() - 1
	p.selectedRow = p.currentPage * p.pageSize
}

// NextRow moves selection down by one row, updating page if necessary.
func (p *Paginator) NextRow() {
	if p.totalItems <= 0 {
		return
	}
	if p.selectedRow < p.totalItems-1 {
		p.selectedRow++
		p.currentPage = p.selectedRow / p.pageSize
	}
}

// PrevRow moves selection up by one row, updating page if necessary.
func (p *Paginator) PrevRow() {
	if p.totalItems <= 0 {
		return
	}
	if p.selectedRow > 0 {
		p.selectedRow--
		p.currentPage = p.selectedRow / p.pageSize
	}
}

// DerivePageSize computes the available page size given terminal height and fixed chrome overhead.
func DerivePageSize(terminalHeight, overhead, fallback int) int {
	if terminalHeight <= 0 {
		return fallback
	}
	available := terminalHeight - overhead
	if available < 1 {
		return 1
	}
	return available
}

// ParseKey maps raw bytes from stdin to a KeyAction and returns number of bytes consumed.
func ParseKey(buf []byte) (KeyAction, int) {
	if len(buf) == 0 {
		return ActionNone, 0
	}

	// 1-byte keys
	switch buf[0] {
	case 'q', 'Q', 0x03: // 'q', 'Q', Ctrl+C
		return ActionQuit, 1
	case 'k', 'K':
		return ActionPrevRow, 1
	case 'j', 'J':
		return ActionNextRow, 1
	case 'h', 'H':
		return ActionPrevPage, 1
	case 'l', 'L', ' ':
		return ActionNextPage, 1
	case 'g':
		return ActionFirstPage, 1
	case 'G':
		return ActionLastPage, 1
	case 0x1b: // Escape sequence or standalone Esc
		if len(buf) == 1 {
			return ActionQuit, 1
		}
		if buf[1] == '[' {
			if len(buf) >= 3 {
				switch buf[2] {
				case 'A': // Up arrow
					return ActionPrevRow, 3
				case 'B': // Down arrow
					return ActionNextRow, 3
				case 'C': // Right arrow
					return ActionNextPage, 3
				case 'D': // Left arrow
					return ActionPrevPage, 3
				case 'H': // Home
					return ActionFirstPage, 3
				case 'F': // End
					return ActionLastPage, 3
				case '5': // Page Up (\x1b[5~)
					if len(buf) >= 4 && buf[3] == '~' {
						return ActionPrevPage, 4
					}
					return ActionPrevPage, 3
				case '6': // Page Down (\x1b[6~)
					if len(buf) >= 4 && buf[3] == '~' {
						return ActionNextPage, 4
					}
					return ActionNextPage, 3
				}
			}
		}
		return ActionQuit, 1
	}

	return ActionNone, 1
}

// ToCRLF ensures all newlines are preceded by a carriage return to prevent staircasing in raw terminal mode.
func ToCRLF(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
}

// Options configures the Pager runner.
type Options struct {
	IsTTY           bool
	In              io.Reader
	Out             io.Writer
	Height          int
	Width           int
	Overhead        int
	DisableRawMode  bool
	AlternateScreen bool
}

// RenderFunc is called on each redraw to format rows.
type RenderFunc func(start, end, selected int, out *bytes.Buffer)

// Pager coordinates viewport sizing, pagination state, and keyboard loop.
type Pager struct {
	opts Options
}

// New creates a new Pager instance.
func New(opts Options) *Pager {
	if opts.In == nil {
		opts.In = os.Stdin
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Overhead == 0 {
		opts.Overhead = 6
	}
	return &Pager{opts: opts}
}

// Run executes the pager display.
func (p *Pager) Run(totalItems int, renderFn RenderFunc) error {
	if totalItems == 0 {
		var buf bytes.Buffer
		renderFn(0, 0, -1, &buf)
		_, err := p.opts.Out.Write(buf.Bytes())
		return err
	}

	if !p.opts.IsTTY {
		// Non-interactive streaming: render all items directly
		var buf bytes.Buffer
		renderFn(0, totalItems, -1, &buf)
		_, err := p.opts.Out.Write(buf.Bytes())
		return err
	}

	// Interactive TTY mode
	height := p.opts.Height
	if height <= 0 {
		if fd := int(os.Stdout.Fd()); term.IsTerminal(fd) {
			_, h, err := term.GetSize(fd)
			if err == nil {
				height = h
			}
		}
	}

	pageSize := DerivePageSize(height, p.opts.Overhead, 15)
	paginator := NewPaginator(totalItems, pageSize)

	if !p.opts.DisableRawMode {
		inFd := int(os.Stdin.Fd())
		if term.IsTerminal(inFd) {
			oldState, err := term.MakeRaw(inFd)
			if err == nil {
				defer func() {
					_ = term.Restore(inFd, oldState)
				}()
			}
		}
	}

	if p.opts.AlternateScreen {
		_, _ = p.opts.Out.Write([]byte("\x1b[?1049h\x1b[H\x1b[?25l"))
		defer func() {
			_, _ = p.opts.Out.Write([]byte("\x1b[?25h\x1b[?1049l"))
		}()
	}

	// Render loop
	var pending []byte
	for {
		if p.opts.Height <= 0 {
			if fd := int(os.Stdout.Fd()); term.IsTerminal(fd) {
				if _, h, err := term.GetSize(fd); err == nil && h > 0 {
					newPageSize := DerivePageSize(h, p.opts.Overhead, 15)
					if newPageSize != paginator.PageSize() {
						paginator.SetPageSize(newPageSize)
					}
				}
			}
		}

		start, end := paginator.PageBounds()
		var screen bytes.Buffer
		// Clear screen and move cursor to top-left
		screen.WriteString("\x1b[H\x1b[2J")
		renderFn(start, end, paginator.SelectedRow(), &screen)

		outBytes := ToCRLF(screen.Bytes())
		if _, err := p.opts.Out.Write(outBytes); err != nil {
			return err
		}

		if len(pending) == 0 {
			buf := make([]byte, 64)
			n, err := p.opts.In.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
			pending = buf[:n]
		}

		action, consumed := ParseKey(pending)
		if consumed > 0 && consumed <= len(pending) {
			pending = pending[consumed:]
		} else {
			pending = nil
		}

		switch action {
		case ActionQuit:
			return nil
		case ActionNextPage:
			paginator.NextPage()
		case ActionPrevPage:
			paginator.PrevPage()
		case ActionNextRow:
			paginator.NextRow()
		case ActionPrevRow:
			paginator.PrevRow()
		case ActionFirstPage:
			paginator.FirstPage()
		case ActionLastPage:
			paginator.LastPage()
		case ActionNone:
		}
	}

	return nil
}
