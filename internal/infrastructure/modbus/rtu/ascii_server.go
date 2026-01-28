package rtu

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
)

// ASCIIServer はModbus RTU ASCIIサーバーを表す
type ASCIIServer struct {
	mu        sync.Mutex
	serial    *ASCIISerialManager
	handler   RequestHandler
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewASCIIServer は新しいASCIIServerを作成する
func NewASCIIServer(config SerialConfig, handler RequestHandler) *ASCIIServer {
	return &ASCIIServer{
		serial:  NewASCIISerialManager(config),
		handler: handler,
	}
}

// Start はサーバーを起動する
func (s *ASCIIServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	if err := s.serial.Open(); err != nil {
		return err
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.running = true

	s.wg.Add(1)
	go s.mainLoop()

	return nil
}

// Stop はサーバーを停止する
func (s *ASCIIServer) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.cancel()
	s.running = false
	s.mu.Unlock()

	// シリアルポートを閉じてReadFrameをアンブロックする
	s.serial.Close()

	// ゴルーチンの終了を待つ
	s.wg.Wait()

	return nil
}

// IsRunning はサーバーが実行中かどうかを返す
func (s *ASCIIServer) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *ASCIIServer) mainLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			s.processNextRequest()
		}
	}
}

func (s *ASCIIServer) processNextRequest() {
	// フレームを読み取る
	frame, err := s.serial.ReadFrame()
	if err != nil {
		// タイムアウトは正常なので無視
		return
	}

	if len(frame) == 0 {
		return
	}

	// リクエストを解析
	req, err := ParseASCIIRequest(frame)
	if err != nil {
		log.Printf("ASCII: failed to parse request: %v", err)
		return
	}

	// UnitIDが無効な場合は応答しない
	if !s.handler.IsUnitIDEnabled(req.UnitID) {
		return
	}

	// リクエストを処理
	response := s.processRequest(req)
	if response == nil {
		return
	}

	// レスポンスを送信
	if err := s.serial.Write(response); err != nil {
		log.Printf("ASCII: failed to write response: %v", err)
	}
}

func (s *ASCIIServer) processRequest(req *Request) []byte {
	switch req.FunctionCode {
	case FuncReadCoils:
		return s.processReadCoils(req)
	case FuncReadDiscreteInputs:
		return s.processReadDiscreteInputs(req)
	case FuncReadHoldingRegisters:
		return s.processReadHoldingRegisters(req)
	case FuncReadInputRegisters:
		return s.processReadInputRegisters(req)
	case FuncWriteSingleCoil:
		return s.processWriteSingleCoil(req)
	case FuncWriteSingleRegister:
		return s.processWriteSingleRegister(req)
	case FuncWriteMultipleCoils:
		return s.processWriteMultipleCoils(req)
	case FuncWriteMultipleRegisters:
		return s.processWriteMultipleRegisters(req)
	default:
		return BuildASCIIExceptionResponse(req.UnitID, req.FunctionCode, ExceptionIllegalFunction)
	}
}

func (s *ASCIIServer) processReadCoils(req *Request) []byte {
	values, err := s.handler.HandleReadCoils(req.UnitID, req.Address, req.Quantity)
	if err != nil {
		return s.buildExceptionFromError(req.UnitID, req.FunctionCode, err)
	}
	return BuildASCIIReadBitsResponse(req.UnitID, req.FunctionCode, values)
}

func (s *ASCIIServer) processReadDiscreteInputs(req *Request) []byte {
	values, err := s.handler.HandleReadDiscreteInputs(req.UnitID, req.Address, req.Quantity)
	if err != nil {
		return s.buildExceptionFromError(req.UnitID, req.FunctionCode, err)
	}
	return BuildASCIIReadBitsResponse(req.UnitID, req.FunctionCode, values)
}

func (s *ASCIIServer) processReadHoldingRegisters(req *Request) []byte {
	values, err := s.handler.HandleReadHoldingRegisters(req.UnitID, req.Address, req.Quantity)
	if err != nil {
		return s.buildExceptionFromError(req.UnitID, req.FunctionCode, err)
	}
	return BuildASCIIReadRegistersResponse(req.UnitID, req.FunctionCode, values)
}

func (s *ASCIIServer) processReadInputRegisters(req *Request) []byte {
	values, err := s.handler.HandleReadInputRegisters(req.UnitID, req.Address, req.Quantity)
	if err != nil {
		return s.buildExceptionFromError(req.UnitID, req.FunctionCode, err)
	}
	return BuildASCIIReadRegistersResponse(req.UnitID, req.FunctionCode, values)
}

func (s *ASCIIServer) processWriteSingleCoil(req *Request) []byte {
	if len(req.Data) < 2 {
		return BuildASCIIExceptionResponse(req.UnitID, req.FunctionCode, ExceptionIllegalDataValue)
	}

	value := binary.BigEndian.Uint16(req.Data)
	var boolValue bool
	if value == 0xFF00 {
		boolValue = true
	} else if value == 0x0000 {
		boolValue = false
	} else {
		return BuildASCIIExceptionResponse(req.UnitID, req.FunctionCode, ExceptionIllegalDataValue)
	}

	if err := s.handler.HandleWriteSingleCoil(req.UnitID, req.Address, boolValue); err != nil {
		return s.buildExceptionFromError(req.UnitID, req.FunctionCode, err)
	}

	return BuildASCIIWriteSingleResponse(req.UnitID, req.FunctionCode, req.Address, value)
}

func (s *ASCIIServer) processWriteSingleRegister(req *Request) []byte {
	if len(req.Data) < 2 {
		return BuildASCIIExceptionResponse(req.UnitID, req.FunctionCode, ExceptionIllegalDataValue)
	}

	value := binary.BigEndian.Uint16(req.Data)
	if err := s.handler.HandleWriteSingleRegister(req.UnitID, req.Address, value); err != nil {
		return s.buildExceptionFromError(req.UnitID, req.FunctionCode, err)
	}

	return BuildASCIIWriteSingleResponse(req.UnitID, req.FunctionCode, req.Address, value)
}

func (s *ASCIIServer) processWriteMultipleCoils(req *Request) []byte {
	values := unpackBools(req.Data, int(req.Quantity))
	if err := s.handler.HandleWriteMultipleCoils(req.UnitID, req.Address, values); err != nil {
		return s.buildExceptionFromError(req.UnitID, req.FunctionCode, err)
	}
	return BuildASCIIWriteMultipleResponse(req.UnitID, req.FunctionCode, req.Address, req.Quantity)
}

func (s *ASCIIServer) processWriteMultipleRegisters(req *Request) []byte {
	values := unpackUint16s(req.Data)
	if err := s.handler.HandleWriteMultipleRegisters(req.UnitID, req.Address, values); err != nil {
		return s.buildExceptionFromError(req.UnitID, req.FunctionCode, err)
	}
	return BuildASCIIWriteMultipleResponse(req.UnitID, req.FunctionCode, req.Address, req.Quantity)
}

func (s *ASCIIServer) buildExceptionFromError(unitID, funcCode byte, err error) []byte {
	var exCode byte
	switch err {
	case ErrIllegalFunction:
		exCode = ExceptionIllegalFunction
	case ErrIllegalDataAddress:
		exCode = ExceptionIllegalDataAddress
	case ErrIllegalDataValue:
		exCode = ExceptionIllegalDataValue
	default:
		if me, ok := err.(*ModbusException); ok {
			exCode = me.Code
		} else {
			exCode = ExceptionSlaveDeviceFailure
		}
	}
	return BuildASCIIExceptionResponse(unitID, funcCode, exCode)
}
