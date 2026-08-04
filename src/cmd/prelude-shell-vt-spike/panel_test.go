package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/term"
)

const panelHelperEnv = "PRELUDE_PANEL_TEST_HELPER"

func TestCapturePanelCommandAllocatesTTY(t *testing.T) {
	snapshot := capturePanelHelper(t)
	if got := snapshotText(snapshot); !strings.Contains(got, "tty:true,true,true") {
		t.Fatalf("panel child did not receive TTY stdio: %q", got)
	}
}

func TestPanelCapturePreservesTTYRendering(t *testing.T) {
	snapshot := capturePanelHelper(t)
	red := snapshot.cellAt(4, 2)
	if red == nil || red.Content != "R" || red.Style.Fg == nil {
		t.Fatalf("styled cell at 4,2 = %#v, want red R", red)
	}
	if got := snapshotText(snapshot); !strings.Contains(got, "query:true") {
		t.Fatalf("terminal query did not receive an emulator reply: %q", got)
	}
}

func capturePanelHelper(t *testing.T) *panelSnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	env := replaceEnv(os.Environ(), panelHelperEnv, "1")
	snapshot, err := capturePanelCommand(
		ctx,
		os.Args[0],
		[]string{"-test.run=^TestPanelHelperProcess$"},
		40,
		4,
		env,
	)
	if err != nil {
		t.Fatalf("capture helper: %v", err)
	}
	return snapshot
}

func TestPanelHelperProcess(t *testing.T) {
	if os.Getenv(panelHelperEnv) != "1" {
		return
	}
	fmt.Printf(
		"\x1b[1;1Htty:%t,%t,%t",
		term.IsTerminal(int(os.Stdin.Fd())),
		term.IsTerminal(int(os.Stdout.Fd())),
		term.IsTerminal(int(os.Stderr.Fd())),
	)

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Printf("\x1b[2;1Hquery:false")
	} else {
		fmt.Print("\x1b]11;?\x07")
		var response []byte
		buffer := make([]byte, 64)
		for !bytes.Contains(response, []byte{0x07}) && len(response) < 256 {
			count, readErr := os.Stdin.Read(buffer)
			response = append(response, buffer[:count]...)
			if readErr != nil {
				break
			}
		}
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
		fmt.Printf("\x1b[2;1Hquery:%t", bytes.Contains(response, []byte("\x1b]11;rgb:")))
	}
	fmt.Print("\x1b[3;5H\x1b[31mRED\x1b[0m")
	os.Exit(0)
}

func snapshotText(snapshot *panelSnapshot) string {
	var builder strings.Builder
	for y := 0; y < snapshot.rows; y++ {
		for x := 0; x < snapshot.cols; x++ {
			if cell := snapshot.cellAt(x, y); cell != nil {
				builder.WriteString(cell.Content)
			}
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}
