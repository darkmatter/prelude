package docs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/term"

	"prelude/pkg/manual"
	"prelude/pkg/shared"
)

type printKind uint8

const (
	printKindPage printKind = iota + 1
	printKindNext
	printKindPrev
)

type printRequest struct {
	kind   printKind
	page   int
	offset int
}

func parsePrintArgs(args []string) (printRequest, error) {
	if len(args) == 1 && args[0] == "next" {
		return printRequest{kind: printKindNext}, nil
	}
	if len(args) == 1 && args[0] == "prev" {
		return printRequest{kind: printKindPrev}, nil
	}
	if len(args) < 1 || len(args) > 2 {
		return printRequest{}, usageError()
	}
	page, err := strconv.Atoi(args[0])
	if err != nil || page < 1 {
		return printRequest{}, usageError()
	}
	offset := 0
	if len(args) == 2 {
		offset, err = strconv.Atoi(args[1])
		if err != nil || offset < 0 {
			return printRequest{}, usageError()
		}
	}
	return printRequest{kind: printKindPage, page: page, offset: offset}, nil
}

func usageError() error {
	return fmt.Errorf("docs: usage: docs [<page> [<offset>] | next | prev]")
}

func wrapWidth(cols int) int {
	return max(min(cols, 96), 16)
}

func windowHeight(rows int) int {
	return max(rows-2, 1)
}

type pagerState struct {
	Page   int    `json:"page"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Width  int    `json:"width"`
	Config string `json:"config"`
}

type printEnv struct {
	cols       int
	rows       int
	statePath  string
	configPath string
	stdout     io.Writer
	stderr     io.Writer
	environ    []string
}

func runPrint(cfg *Config, args []string, env printEnv) error {
	request, err := parsePrintArgs(args)
	if err != nil {
		return err
	}
	if env.stdout == nil {
		env.stdout = os.Stdout
	}
	if env.stderr == nil {
		env.stderr = os.Stderr
	}
	if env.cols <= 0 || env.rows <= 0 {
		env.cols, env.rows = terminalSize(env.stdout)
	}
	width, height := wrapWidth(env.cols), windowHeight(env.rows)
	document := manualDocument(cfg)
	state, err := loadPagerState(env.statePath, env.configPath)
	if err != nil {
		state = pagerState{}
	}

	page, start := request.page, request.offset
	switch request.kind {
	case printKindNext, printKindPrev:
		if state.Page == 0 {
			page, start = 1, 0
		} else {
			page = state.Page
			start = state.Start
			if state.Width != width {
				start = rebaseOffset(state.Start, state.Width, width, document, page, cfg.Palette)
			}
			if request.kind == printKindNext {
				start = rebaseOffset(state.End, state.Width, width, document, page, cfg.Palette)
				lines, renderErr := manual.RenderLeafLines(document, cfg.Palette, page, width)
				if renderErr != nil {
					return renderErr
				}
				if start >= len(lines) {
					if page < manual.LeafCount(document) {
						page++
						start = 0
					} else {
						start = lastWindowStart(len(lines), height)
					}
				}
			} else if start <= 0 {
				if page > 1 {
					page--
					lines, renderErr := manual.RenderLeafLines(document, cfg.Palette, page, width)
					if renderErr != nil {
						return renderErr
					}
					start = lastWindowStart(len(lines), height)
				} else {
					start = 0
				}
			} else {
				start = max(0, start-height)
			}
		}
	}

	lines, err := manual.RenderLeafLines(document, cfg.Palette, page, width)
	if err != nil {
		return err
	}
	start = min(start, lastWindowStart(len(lines), height))
	end := min(start+height, len(lines))
	writer := shared.ColorWriter(env.stdout, env.environ, cfg.ColorProfile)
	for _, line := range lines[start:end] {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
	}
	if env.statePath != "" {
		return savePagerState(env.statePath, pagerState{Page: page, Start: start, End: end, Width: width, Config: env.configPath})
	}
	return nil
}

// rebaseOffset maps a wrapped-line cursor between widths. Character-level
// mapping would require retaining the renderer's internal layout; proportional
// mapping preserves the reader's position without coupling the pager to it.
func rebaseOffset(offset, oldWidth, width int, document manual.Document, page int, palette shared.Palette) int {
	if oldWidth <= 0 || oldWidth == width {
		return max(offset, 0)
	}
	oldLines, oldErr := manual.RenderLeafLines(document, palette, page, oldWidth)
	newLines, newErr := manual.RenderLeafLines(document, palette, page, width)
	if oldErr != nil || newErr != nil || len(oldLines) == 0 {
		return 0
	}
	return min(max(offset*len(newLines)/len(oldLines), 0), len(newLines))
}

func lastWindowStart(lineCount, height int) int {
	return max(lineCount-height, 0)
}

func terminalSize(output io.Writer) (int, int) {
	if file, ok := output.(interface{ Fd() uintptr }); ok {
		if cols, rows, err := term.GetSize(int(file.Fd())); err == nil && cols > 0 && rows > 0 {
			return cols, rows
		}
	}
	return 80, 24
}

func loadPagerState(path, config string) (pagerState, error) {
	if path == "" {
		return pagerState{}, os.ErrNotExist
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return pagerState{}, err
	}
	var state pagerState
	if err := json.Unmarshal(raw, &state); err != nil || state.Config != config || state.Page < 1 {
		return pagerState{}, os.ErrInvalid
	}
	return state, nil
}

func savePagerState(path string, state pagerState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".docs-print-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func defaultPagerStatePath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	tty := "notty"
	if target, err := os.Readlink("/dev/fd/1"); err == nil && target != "" {
		tty = filepath.Base(target)
	}
	return filepath.Join(dir, "prelude", "docs-print-"+strconv.Itoa(os.Getuid())+"-"+tty)
}
