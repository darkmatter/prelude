// libprelude: prelude's menu, MOTD, and docs as a C shared library for hosts
// that run them in their own process. The TypeScript API (ts/) loads it through
// bun:ffi, which also lets `bun build --compile` embed it in an executable.
//
//	go build -buildmode=c-shared -o libprelude.so ./cmd/libprelude
//
// Every export takes one JSON request as a NUL-terminated string and returns
// one JSON reply, {"ok": <result>} or {"error": "<message>"}, that the caller
// releases with prelude_free. Exports never exit or exec because the host owns
// the process; a panic is recovered into an error reply for the same reason.
package main

// #include <stdlib.h>
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"prelude/internal/docs"
	"prelude/internal/menu"
	"prelude/internal/motd"
	"prelude/pkg/shared"
)

// abiVersion changes whenever a request or reply shape does, so a host can
// refuse a library it does not speak instead of misreading it.
const abiVersion = 2

//export prelude_abi_version
func prelude_abi_version() C.int {
	return abiVersion
}

//export prelude_free
func prelude_free(reply *C.char) {
	C.free(unsafe.Pointer(reply))
}

// prelude_menu_select resolves args like `x`, opening the picker where input
// is still needed. Replies with the selection, or null when the user leaves.
//
//export prelude_menu_select
func prelude_menu_select(request *C.char) *C.char {
	return handle(request, func(req struct {
		Config json.RawMessage `json:"config"`
		Args   []string        `json:"args"`
	}) (any, error) {
		cfg, err := menu.ParseConfig(req.Config)
		if err != nil {
			return nil, err
		}
		return menu.Select(cfg, req.Args)
	})
}

// prelude_menu_list replies with the `x --list` table for a terminal width.
//
//export prelude_menu_list
func prelude_menu_list(request *C.char) *C.char {
	return handle(request, func(req struct {
		Config json.RawMessage `json:"config"`
		Width  int             `json:"width"`
	}) (any, error) {
		cfg, err := menu.ParseConfig(req.Config)
		if err != nil {
			return nil, err
		}
		var out strings.Builder
		menu.List(&out, os.Stdout, os.Environ(), cfg, req.Width)
		return out.String(), nil
	})
}

// prelude_motd_render replies with the banner, after running the configured
// shell checks and probes and recording the host's own results.
//
//export prelude_motd_render
func prelude_motd_render(request *C.char) *C.char {
	return handle(request, func(req struct {
		Config  json.RawMessage `json:"config"`
		Results motd.Results    `json:"results"`
		Width   int             `json:"width"`
		Height  int             `json:"height"`
	}) (any, error) {
		cfg, err := motd.ParseConfig(req.Config)
		if err != nil {
			return nil, err
		}
		return forStdout(cfg.ColorProfile, motd.RenderHosted(cfg, req.Results, req.Width, req.Height)), nil
	})
}

type pageReply struct {
	Output string `json:"output"`
	Pages  int    `json:"pages"`
}

// prelude_docs_render replies with one page as `docs <page>` prints it, plus
// the page count so the host can page through the document.
//
//export prelude_docs_render
func prelude_docs_render(request *C.char) *C.char {
	return handle(request, func(req struct {
		Config json.RawMessage `json:"config"`
		Page   int             `json:"page"`
		Width  int             `json:"width"`
	}) (any, error) {
		cfg, err := docs.ParseConfig(req.Config)
		if err != nil {
			return nil, err
		}
		lines, pages, err := docs.RenderPage(cfg, req.Page, req.Width)
		if err != nil {
			return nil, err
		}
		return pageReply{Output: forStdout(cfg.ColorProfile, strings.Join(lines, "\n")), Pages: pages}, nil
	})
}

// prelude_docs_open runs the full-screen docs viewer until the user quits.
//
//export prelude_docs_open
func prelude_docs_open(request *C.char) *C.char {
	return handle(request, func(req struct {
		Config json.RawMessage `json:"config"`
	}) (any, error) {
		cfg, err := docs.ParseConfig(req.Config)
		if err != nil {
			return nil, err
		}
		return nil, docs.View(cfg)
	})
}

// handle decodes a strict request, runs it, and encodes the reply, turning a
// panic into an error reply rather than a crash of the host process.
func handle[Request any](request *C.char, run func(Request) (any, error)) (reply *C.char) {
	defer func() {
		if recovered := recover(); recovered != nil {
			reply = encode(nil, fmt.Errorf("panic: %v", recovered))
		}
	}()
	req, err := shared.DecodeJSON[Request]([]byte(C.GoString(request)))
	if err != nil {
		return encode(nil, fmt.Errorf("request: %w", err))
	}
	return encode(run(*req))
}

func encode(result any, err error) *C.char {
	var reply any = map[string]any{"ok": result}
	if err != nil {
		reply = map[string]string{"error": err.Error()}
	}
	raw, err := json.Marshal(reply)
	if err != nil {
		raw, _ = json.Marshal(map[string]string{"error": err.Error()})
	}
	return C.CString(string(raw))
}

// forStdout downgrades rendered ANSI for this process's stdout, where the host
// prints what it receives.
func forStdout(profile, rendered string) string {
	var out strings.Builder
	_, _ = shared.ColorWriterFor(&out, os.Stdout, os.Environ(), profile).Write([]byte(rendered))
	return out.String()
}

func main() {}
