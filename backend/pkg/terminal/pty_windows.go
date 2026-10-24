//go:build windows

package terminal

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsPTYProcess struct {
	console, process     windows.Handle
	input, reader        *os.File
	output               chan []byte
	done, stop           chan struct{}
	once                 sync.Once
	consoleMu, processMu sync.Mutex
	exitCode             int
	waitErr              error
}

// startPTYProcess attaches the child to a native Windows pseudoconsole.
func startPTYProcess(ctx context.Context, params CreateParams) (ptyProcess, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if err := windows.NewLazySystemDLL("kernel32.dll").NewProc("CreatePseudoConsole").Find(); err != nil {
		return nil, 0, fmt.Errorf("terminal: Windows ConPTY is unavailable; run without use_pty: %w", err)
	}
	cmd := buildCommand(ctx, params)
	if cmd.Err != nil {
		return nil, 0, cmd.Err
	}
	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		return nil, 0, err
	}
	defer inputRead.Close()
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		inputWrite.Close()
		return nil, 0, err
	}
	defer outputWrite.Close()
	success := false
	defer func() {
		if !success {
			inputWrite.Close()
			outputRead.Close()
		}
	}()
	rows, cols := params.Rows, params.Cols
	if rows == 0 {
		rows = 40
	}
	if cols == 0 {
		cols = 120
	}
	var console windows.Handle
	err = windows.CreatePseudoConsole(windows.Coord{X: int16(cols), Y: int16(rows)}, windows.Handle(inputRead.Fd()), windows.Handle(outputWrite.Fd()), 0, &console)
	if err != nil {
		return nil, 0, fmt.Errorf("terminal: create ConPTY: %w", err)
	}
	defer func() {
		if !success {
			windows.ClosePseudoConsole(console)
		}
	}()
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, 0, err
	}
	defer attributes.Delete()
	// This attribute takes the opaque HPCON value itself, not a pointer to it.
	update := windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")
	ok, _, attrErr := update.Call(uintptr(unsafe.Pointer(attributes.List())), 0, windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(console), unsafe.Sizeof(console), 0, 0)
	if ok == 0 {
		return nil, 0, fmt.Errorf("terminal: attach ConPTY: %w", attrErr)
	}
	startup := windows.StartupInfoEx{StartupInfo: windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})), Flags: windows.STARTF_USESTDHANDLES}, ProcThreadAttributeList: attributes.List()}
	application, err := windows.UTF16PtrFromString(cmd.Path)
	if err != nil {
		return nil, 0, err
	}
	command, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(cmd.Args))
	if err != nil {
		return nil, 0, err
	}
	var cwd *uint16
	if cmd.Dir != "" {
		cwd, err = windows.UTF16PtrFromString(cmd.Dir)
		if err != nil {
			return nil, 0, err
		}
	}
	var environment *uint16
	if params.Env != nil {
		env := append([]string(nil), params.Env...)
		sort.Slice(env, func(i, j int) bool { return strings.ToUpper(env[i]) < strings.ToUpper(env[j]) })
		for _, entry := range env {
			if strings.ContainsRune(entry, 0) {
				return nil, 0, fmt.Errorf("terminal: environment contains NUL")
			}
		}
		block := utf16.Encode([]rune(strings.Join(env, "\x00") + "\x00\x00"))
		environment = &block[0]
	}
	var info windows.ProcessInformation
	err = windows.CreateProcess(application, command, nil, nil, false, windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT, environment, cwd, &startup.StartupInfo, &info)
	if err != nil {
		return nil, 0, fmt.Errorf("terminal: start ConPTY process: %w", err)
	}
	windows.CloseHandle(info.Thread)
	p := &windowsPTYProcess{console: console, process: info.Process, input: inputWrite, reader: outputRead, output: make(chan []byte, 64), done: make(chan struct{}), stop: make(chan struct{})}
	success = true
	go p.run()
	go func() {
		select {
		case <-ctx.Done():
			_ = p.Close()
		case <-p.done:
		}
	}()
	return p, int(info.ProcessId), nil
}

func (p *windowsPTYProcess) run() {
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		for {
			n, err := p.reader.Read(buf)
			if n > 0 {
				select {
				case p.output <- append([]byte(nil), buf[:n]...):
				case <-p.stop:
				}
			}
			if err != nil {
				return
			}
		}
	}()
	_, p.waitErr = windows.WaitForSingleObject(p.process, windows.INFINITE)
	var code uint32
	if err := windows.GetExitCodeProcess(p.process, &code); p.waitErr == nil {
		p.waitErr = err
	}
	p.exitCode = int(code)
	p.closeConsole() // Keep draining concurrently while ConPTY flushes its final frame.
	<-readDone
	p.reader.Close()
	p.input.Close()
	p.processMu.Lock()
	windows.CloseHandle(p.process)
	p.process = 0
	p.processMu.Unlock()
	close(p.output)
	close(p.done)
}

func (p *windowsPTYProcess) closeConsole() {
	p.consoleMu.Lock()
	defer p.consoleMu.Unlock()
	if p.console != 0 {
		windows.ClosePseudoConsole(p.console)
		p.console = 0
	}
}

func (p *windowsPTYProcess) Output() <-chan []byte   { return p.output }
func (p *windowsPTYProcess) Done() <-chan struct{}   { return p.done }
func (p *windowsPTYProcess) Write(data []byte) error { _, err := p.input.Write(data); return err }
func (p *windowsPTYProcess) Resize(rows, cols uint16) error {
	p.consoleMu.Lock()
	defer p.consoleMu.Unlock()
	if p.console == 0 {
		return fmt.Errorf("terminal: session already closed")
	}
	return windows.ResizePseudoConsole(p.console, windows.Coord{X: int16(cols), Y: int16(rows)})
}
func (p *windowsPTYProcess) Wait() (int, error) { <-p.done; return p.exitCode, p.waitErr }
func (p *windowsPTYProcess) Close() error {
	p.once.Do(func() {
		close(p.stop)
		p.processMu.Lock()
		if p.process != 0 {
			_ = windows.TerminateProcess(p.process, 1)
		}
		p.processMu.Unlock()
		p.closeConsole()
		p.input.Close()
	})
	<-p.done
	return nil
}
