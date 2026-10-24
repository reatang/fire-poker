package terminal

import (
	"context"
	"os"
	"os/exec"
	"sync"
)

// pipeProcess preserves terminal persistence and streaming without requiring a PTY.
type pipeProcess struct {
	cmd           *exec.Cmd
	input, reader *os.File
	output        chan []byte
	done, stop    chan struct{}
	once          sync.Once
	exitCode      int
	waitErr       error
}

func startPipeProcess(ctx context.Context, params CreateParams) (ptyProcess, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	cmd := buildCommand(ctx, params)
	configurePipeCommand(cmd)
	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		return nil, 0, err
	}
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		inputRead.Close()
		inputWrite.Close()
		return nil, 0, err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = inputRead, outputWrite, outputWrite
	if err := cmd.Start(); err != nil {
		inputRead.Close()
		inputWrite.Close()
		outputRead.Close()
		outputWrite.Close()
		return nil, 0, err
	}
	inputRead.Close()
	outputWrite.Close()
	p := &pipeProcess{cmd: cmd, input: inputWrite, reader: outputRead, output: make(chan []byte, 64), done: make(chan struct{}), stop: make(chan struct{})}
	go p.run()
	go func() {
		select {
		case <-ctx.Done():
			_ = p.Close()
		case <-p.done:
		}
	}()
	return p, cmd.Process.Pid, nil
}

func (p *pipeProcess) run() {
	defer close(p.done)
	defer close(p.output)
	defer p.reader.Close()
	defer p.input.Close()
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
			break
		}
	}
	p.waitErr = p.cmd.Wait()
	p.exitCode = p.cmd.ProcessState.ExitCode()
}

func (p *pipeProcess) Output() <-chan []byte          { return p.output }
func (p *pipeProcess) Done() <-chan struct{}          { return p.done }
func (p *pipeProcess) Write(data []byte) error        { _, err := p.input.Write(data); return err }
func (p *pipeProcess) Resize(rows, cols uint16) error { return nil }
func (p *pipeProcess) Wait() (int, error)             { <-p.done; return p.exitCode, p.waitErr }
func (p *pipeProcess) Close() error {
	p.once.Do(func() { close(p.stop); _ = p.cmd.Process.Kill(); _ = p.reader.Close(); _ = p.input.Close() })
	<-p.done
	return nil
}
