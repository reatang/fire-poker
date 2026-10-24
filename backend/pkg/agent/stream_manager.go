package agent

import (
	"context"
	"fmt"
	"sync"
)

type activeStream struct {
	cancel    context.CancelFunc
	confirmCh chan ConfirmResponse
	done      chan struct{}
}

// ConfirmResponse carries the user's response to a tool confirmation request.
type ConfirmResponse struct {
	Approved bool
	Message  string
	Action   string
}

type StreamManager struct {
	mu      sync.RWMutex
	streams map[uint]*activeStream
}

func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[uint]*activeStream),
	}
}

func (sm *StreamManager) Start(parentCtx context.Context, sessionID uint) (context.Context, context.CancelFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if existing, ok := sm.streams[sessionID]; ok {
		existing.cancel()
	}

	ctx, cancel := context.WithCancel(parentCtx)
	sm.streams[sessionID] = &activeStream{
		cancel:    cancel,
		confirmCh: make(chan ConfirmResponse, 1),
		done:      make(chan struct{}),
	}
	return ctx, cancel
}

func (sm *StreamManager) Stop(sessionID uint) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.streams[sessionID]; ok {
		s.cancel()
	}
}

func (sm *StreamManager) Remove(sessionID uint) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.streams[sessionID]; ok {
		s.cancel()
		close(s.done)
		delete(sm.streams, sessionID)
	}
}

// TryStart reserves a turn before history is read or any message is persisted.
func (sm *StreamManager) TryStart(parent context.Context, sessionID uint) (context.Context, context.CancelFunc, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.streams[sessionID]; ok {
		return nil, nil, fmt.Errorf("当前会话仍在生成或保存，请稍后再试")
	}
	ctx, cancel := context.WithCancel(parent)
	sm.streams[sessionID] = &activeStream{cancel: cancel, confirmCh: make(chan ConfirmResponse, 1), done: make(chan struct{})}
	return ctx, cancel, nil
}

// StopAndWait returns only after the consumer has persisted its partial output.
func (sm *StreamManager) StopAndWait(ctx context.Context, sessionID uint) error {
	sm.mu.RLock()
	s := sm.streams[sessionID]
	sm.mu.RUnlock()
	if s == nil {
		return nil
	}
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (sm *StreamManager) IsActive(sessionID uint) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	_, ok := sm.streams[sessionID]
	return ok
}

// WhileInactive serializes a settings write with TryStart, closing the first-turn reservation race.
func (sm *StreamManager) WhileInactive(sessionID uint, update func() error) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, active := sm.streams[sessionID]; active {
		return fmt.Errorf("对话正在开始或生成，不能修改工具选择")
	}
	return update()
}

func (sm *StreamManager) SendConfirmResponse(sessionID uint, resp ConfirmResponse) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s, ok := sm.streams[sessionID]
	if !ok {
		return false
	}
	select {
	case s.confirmCh <- resp:
		return true
	default:
		return false
	}
}

func (sm *StreamManager) WaitForConfirm(ctx context.Context, sessionID uint) (ConfirmResponse, error) {
	sm.mu.RLock()
	s, ok := sm.streams[sessionID]
	sm.mu.RUnlock()

	if !ok {
		return ConfirmResponse{}, context.Canceled
	}

	select {
	case <-ctx.Done():
		return ConfirmResponse{}, ctx.Err()
	case resp, chanOpen := <-s.confirmCh:
		if !chanOpen {
			return ConfirmResponse{}, context.Canceled
		}
		return resp, nil
	}
}
