package application

import (
	"context"
	"sync"
	"time"

	"modbus_simulator/internal/domain/variable"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AppStateEmitter はアプリケーション状態変化イベントを発行するインターフェース
type AppStateEmitter interface {
	EmitServerChanged(instances []ServerInstanceDTO, protocols []ProtocolInfoDTO)
	EmitVariablesChanged(variables []*VariableDTO)
	EmitScriptsChanged(scripts []*ScriptDTO)
	EmitConsoleLogAdded(entry ConsoleLogDTO)
}

// WailsAppStateEmitter はWailsランタイムを使用したAppStateEmitter実装
type WailsAppStateEmitter struct {
	ctx context.Context
}

// NewWailsAppStateEmitter は新しいWailsAppStateEmitterを作成する
func NewWailsAppStateEmitter(ctx context.Context) *WailsAppStateEmitter {
	return &WailsAppStateEmitter{ctx: ctx}
}

// EmitServerChanged はサーバー状態変化イベントを発行する
func (e *WailsAppStateEmitter) EmitServerChanged(instances []ServerInstanceDTO, protocols []ProtocolInfoDTO) {
	if e.ctx == nil {
		return
	}
	runtime.EventsEmit(e.ctx, "plc:server-changed", instances)
	runtime.EventsEmit(e.ctx, "plc:protocols-changed", protocols)
}

// EmitVariablesChanged は変数一覧変化イベントを発行する
func (e *WailsAppStateEmitter) EmitVariablesChanged(variables []*VariableDTO) {
	if e.ctx == nil {
		return
	}
	runtime.EventsEmit(e.ctx, "plc:variables-changed", variables)
}

// EmitScriptsChanged はスクリプト一覧変化イベントを発行する
func (e *WailsAppStateEmitter) EmitScriptsChanged(scripts []*ScriptDTO) {
	if e.ctx == nil {
		return
	}
	runtime.EventsEmit(e.ctx, "plc:scripts-changed", scripts)
}

// EmitConsoleLogAdded はコンソールログ追加イベントを発行する
func (e *WailsAppStateEmitter) EmitConsoleLogAdded(entry ConsoleLogDTO) {
	if e.ctx == nil {
		return
	}
	runtime.EventsEmit(e.ctx, "plc:console-log-added", entry)
}

// variableChangeListener は VariableStore の変更を受け取りスロットルしてイベント発行するリスナー。
// Leading+trailing throttle: 最初の変更で即座に発火し、その後 throttle 期間内の変更は
// 期間終了後に1回まとめて trailing 発火する。
type variableChangeListener struct {
	emitFn   func()
	mu       sync.Mutex
	timer    *time.Timer
	pending  bool // スロットル期間中に変更があったか
	throttle time.Duration
}

// newVariableChangeListener は新しい variableChangeListener を作成する
func newVariableChangeListener(emitFn func(), throttle time.Duration) *variableChangeListener {
	return &variableChangeListener{
		emitFn:   emitFn,
		throttle: throttle,
	}
}

// OnVariableChanged は変数変更通知を受け取り、スロットルして発行する。
// タイマーが動いていなければ即座に発火（leading edge）してタイマーを開始する。
// タイマー動作中は pending フラグを立てるだけ。タイマー満了時に pending があれば trailing 発火する。
func (l *variableChangeListener) OnVariableChanged(_ *variable.Variable, _ []variable.ProtocolMapping) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.timer != nil {
		// スロットル期間中: trailing 発火フラグをセット
		l.pending = true
		return
	}
	// Leading edge: 即座に発火
	go l.emitFn()
	// スロットル期間を開始
	l.timer = time.AfterFunc(l.throttle, func() {
		l.mu.Lock()
		needsTrailing := l.pending
		l.pending = false
		l.timer = nil
		l.mu.Unlock()
		if needsTrailing {
			l.emitFn()
		}
	})
}
