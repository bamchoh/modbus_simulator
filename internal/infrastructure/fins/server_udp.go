package fins

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"modbus_simulator/internal/domain/protocol"
)

// FINSUDPServer はFINS/UDPサーバー
type FINSUDPServer struct {
	config       *FINSConfig
	store        *FINSDataStore
	handler      *Handler
	conn         *net.UDPConn
	status       protocol.ServerStatus
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	activePeers  map[string]time.Time // ピアのアクティブ状態を追跡
	eventEmitter protocol.CommunicationEventEmitter
}

// NewFINSUDPServer は新しいFINSUDPServerを作成する
func NewFINSUDPServer(config *FINSConfig, store *FINSDataStore) *FINSUDPServer {
	return &FINSUDPServer{
		config:      config,
		store:       store,
		handler:     NewHandler(store, config.NodeAddress, config.NetworkID),
		status:      protocol.StatusStopped,
		activePeers: make(map[string]time.Time),
	}
}

// Start はサーバーを起動する
func (s *FINSUDPServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == protocol.StatusRunning {
		return fmt.Errorf("server is already running")
	}

	addr := fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		s.status = protocol.StatusError
		return fmt.Errorf("failed to resolve UDP address %s: %v", addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		s.status = protocol.StatusError
		return fmt.Errorf("failed to listen on UDP %s: %v", addr, err)
	}

	s.conn = conn
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.status = protocol.StatusRunning

	// 受信ループ
	s.wg.Add(1)
	go s.receiveLoop()

	// ピアタイムアウト監視
	s.wg.Add(1)
	go s.peerTimeoutMonitor()

	log.Printf("FINS/UDP server started on %s", addr)
	return nil
}

// Stop はサーバーを停止する
func (s *FINSUDPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != protocol.StatusRunning {
		return nil
	}

	// キャンセルを通知
	if s.cancel != nil {
		s.cancel()
	}

	// コネクションを閉じる
	if s.conn != nil {
		s.conn.Close()
	}

	// ゴルーチンの終了を待つ
	s.wg.Wait()

	s.status = protocol.StatusStopped
	s.activePeers = make(map[string]time.Time)

	log.Println("FINS/UDP server stopped")
	return nil
}

// Status はサーバーの状態を返す
func (s *FINSUDPServer) Status() protocol.ServerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// ProtocolType はプロトコルの種類を返す
func (s *FINSUDPServer) ProtocolType() protocol.ProtocolType {
	return protocol.ProtocolFINS
}

// Config は現在の設定を返す
func (s *FINSUDPServer) Config() protocol.ProtocolConfig {
	return s.config
}

// UpdateConfig は設定を更新する
func (s *FINSUDPServer) UpdateConfig(config protocol.ProtocolConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == protocol.StatusRunning {
		return fmt.Errorf("cannot update config while server is running")
	}

	finsConfig, ok := config.(*FINSConfig)
	if !ok {
		return fmt.Errorf("invalid config type: expected FINSConfig")
	}

	s.config = finsConfig
	s.handler.SetNodeAddress(finsConfig.NodeAddress)
	s.handler.SetNetworkID(finsConfig.NetworkID)
	return nil
}

// SetEventEmitter はイベントエミッターを設定する
func (s *FINSUDPServer) SetEventEmitter(emitter protocol.CommunicationEventEmitter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventEmitter = emitter
}

// emitConnection は接続数変更イベントを発行する
func (s *FINSUDPServer) emitConnection() {
	if s.eventEmitter != nil {
		s.eventEmitter.EmitConnection(len(s.activePeers))
	}
}

// receiveLoop はUDPパケットを受信するループ
func (s *FINSUDPServer) receiveLoop() {
	defer s.wg.Done()

	buf := make([]byte, 4096)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// タイムアウト設定（コンテキストキャンセルを検出するため）
		s.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		n, remoteAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Printf("FINS/UDP: Read error: %v", err)
				continue
			}
		}

		// 受信イベント発行
		if s.eventEmitter != nil {
			s.eventEmitter.EmitRx()
		}

		// ピアを追跡
		peerKey := remoteAddr.String()
		s.mu.Lock()
		_, existed := s.activePeers[peerKey]
		s.activePeers[peerKey] = time.Now()
		if !existed {
			s.emitConnection()
		}
		s.mu.Unlock()

		// フレームを処理
		response := s.processFrame(buf[:n])
		if response != nil {
			_, err := s.conn.WriteToUDP(response, remoteAddr)
			if err != nil {
				log.Printf("FINS/UDP: Write error: %v", err)
				continue
			}
			// 送信イベント発行
			if s.eventEmitter != nil {
				s.eventEmitter.EmitTx()
			}
		}
	}
}

// peerTimeoutMonitor は非アクティブなピアを削除する
func (s *FINSUDPServer) peerTimeoutMonitor() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeout := 30 * time.Second

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			changed := false
			for peer, lastSeen := range s.activePeers {
				if now.Sub(lastSeen) > timeout {
					delete(s.activePeers, peer)
					changed = true
				}
			}
			if changed {
				s.emitConnection()
			}
			s.mu.Unlock()
		}
	}
}

// processFrame はFINS/UDPフレームを処理する
func (s *FINSUDPServer) processFrame(data []byte) []byte {
	// FINS/UDPはTCPヘッダーがなく、コマンドヘッダーから始まる
	frame, err := ParseUDPFrame(data)
	if err != nil {
		log.Printf("FINS/UDP: Failed to parse frame: %v", err)
		return nil
	}

	// FINSコマンドを処理
	return s.handler.HandleUDPCommand(frame)
}
