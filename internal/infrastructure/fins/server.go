package fins

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"modbus_simulator/internal/domain/protocol"
)

// FINSServer はFINS/TCPサーバー
type FINSServer struct {
	config       *FINSConfig
	store        *FINSDataStore
	handler      *Handler
	listener     net.Listener
	status       protocol.ServerStatus
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	clients      map[net.Conn]byte // conn -> assigned client node
	eventEmitter protocol.CommunicationEventEmitter
}

// NewFINSServer は新しいFINSServerを作成する
func NewFINSServer(config *FINSConfig, store *FINSDataStore) *FINSServer {
	return &FINSServer{
		config:  config,
		store:   store,
		handler: NewHandler(store, config.NodeAddress, config.NetworkID),
		status:  protocol.StatusStopped,
		clients: make(map[net.Conn]byte),
	}
}

// Start はサーバーを起動する
func (s *FINSServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == protocol.StatusRunning {
		return fmt.Errorf("server is already running")
	}

	addr := fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.status = protocol.StatusError
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}

	s.listener = listener
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.status = protocol.StatusRunning

	// 接続受付ゴルーチン
	s.wg.Add(1)
	go s.acceptLoop()

	log.Printf("FINS server started on %s", addr)
	return nil
}

// Stop はサーバーを停止する
func (s *FINSServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != protocol.StatusRunning {
		return nil
	}

	// キャンセルを通知
	if s.cancel != nil {
		s.cancel()
	}

	// リスナーを閉じる
	if s.listener != nil {
		s.listener.Close()
	}

	// 全クライアント接続を閉じる
	for conn := range s.clients {
		conn.Close()
	}

	// ゴルーチンの終了を待つ
	s.wg.Wait()

	s.status = protocol.StatusStopped
	s.clients = make(map[net.Conn]byte)

	log.Println("FINS server stopped")
	return nil
}

// Status はサーバーの状態を返す
func (s *FINSServer) Status() protocol.ServerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// ProtocolType はプロトコルの種類を返す
func (s *FINSServer) ProtocolType() protocol.ProtocolType {
	return protocol.ProtocolFINS
}

// Config は現在の設定を返す
func (s *FINSServer) Config() protocol.ProtocolConfig {
	return s.config
}

// UpdateConfig は設定を更新する
func (s *FINSServer) UpdateConfig(config protocol.ProtocolConfig) error {
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
func (s *FINSServer) SetEventEmitter(emitter protocol.CommunicationEventEmitter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventEmitter = emitter
}

// emitConnection は接続数変更イベントを発行する
func (s *FINSServer) emitConnection() {
	if s.eventEmitter != nil {
		s.eventEmitter.EmitConnection(len(s.clients))
	}
}

// acceptLoop は接続を受け付けるループ
func (s *FINSServer) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Printf("FINS: Accept error: %v", err)
				continue
			}
		}

		s.mu.Lock()
		s.clients[conn] = 0 // クライアントノード未割り当て
		s.emitConnection()
		s.mu.Unlock()

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection は個別の接続を処理する
func (s *FINSServer) handleConnection(conn net.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.emitConnection()
		s.mu.Unlock()
		conn.Close()
		s.wg.Done()
	}()

	log.Printf("FINS: New connection from %s", conn.RemoteAddr())

	buf := make([]byte, 4096)
	var accumulated []byte

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Printf("FINS: Connection closed by client: %s", conn.RemoteAddr())
			} else {
				select {
				case <-s.ctx.Done():
					return
				default:
					log.Printf("FINS: Read error: %v", err)
				}
			}
			return
		}

		// 受信イベント発行
		if s.eventEmitter != nil {
			s.eventEmitter.EmitRx()
		}

		accumulated = append(accumulated, buf[:n]...)

		// 完全なフレームを処理
		for len(accumulated) >= FINSTCPHeaderSize {
			// TCPヘッダーをパース
			header, err := ParseTCPHeader(accumulated)
			if err != nil {
				log.Printf("FINS: Invalid header: %v", err)
				accumulated = accumulated[1:] // 1バイトスキップして再試行
				continue
			}

			frameLen := FINSTCPHeaderSize + int(header.Length)
			if len(accumulated) < frameLen {
				// フレームが不完全
				break
			}

			frameData := accumulated[:frameLen]
			accumulated = accumulated[frameLen:]

			// フレームを処理
			response := s.processFrame(frameData)
			if response != nil {
				_, err := conn.Write(response)
				if err != nil {
					log.Printf("FINS: Write error: %v", err)
					return
				}
				// 送信イベント発行
				if s.eventEmitter != nil {
					s.eventEmitter.EmitTx()
				}
			}
		}
	}
}

// processFrame はFINSフレームを処理する
func (s *FINSServer) processFrame(data []byte) []byte {
	frame, err := ParseFrame(data)
	if err != nil {
		log.Printf("FINS: Failed to parse frame: %v", err)
		return nil
	}

	// ノードアドレス送信コマンドの場合
	if frame.TCPHeader.Command == TCPCmdNodeAddressSend {
		return s.handler.HandleNodeAddressRequest(frame.Data)
	}

	// 通常のFINSコマンドの場合
	if frame.TCPHeader.Command == TCPCmdFrameSend {
		return s.handler.HandleCommand(frame)
	}

	log.Printf("FINS: Unknown TCP command: %d", frame.TCPHeader.Command)
	return nil
}
