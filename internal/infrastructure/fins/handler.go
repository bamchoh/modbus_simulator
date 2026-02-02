package fins

import (
	"encoding/binary"
	"log"
)

// Handler はFINSコマンドを処理するハンドラー
type Handler struct {
	store      *FINSDataStore
	nodeAddr   byte
	networkID  byte
}

// NewHandler は新しいHandlerを作成する
func NewHandler(store *FINSDataStore, nodeAddr, networkID byte) *Handler {
	return &Handler{
		store:     store,
		nodeAddr:  nodeAddr,
		networkID: networkID,
	}
}

// HandleCommand はFINSコマンドを処理し、レスポンスを返す
func (h *Handler) HandleCommand(frame *Frame) []byte {
	if frame.Header == nil {
		return nil
	}

	cmdCode := frame.Command.Code()

	switch cmdCode {
	case CmdMemoryAreaRead:
		return h.handleMemoryAreaRead(frame)
	case CmdMemoryAreaWrite:
		return h.handleMemoryAreaWrite(frame)
	default:
		log.Printf("FINS: Unknown command: 0x%04X", cmdCode)
		return BuildErrorResponse(frame.Header, frame.Command, ErrCommandNotSupported)
	}
}

// handleMemoryAreaRead はメモリエリア読み込みコマンドを処理する
func (h *Handler) handleMemoryAreaRead(frame *Frame) []byte {
	req, err := ParseMemoryAreaReadRequest(frame.Data)
	if err != nil {
		log.Printf("FINS: Failed to parse memory read request: %v", err)
		return BuildErrorResponse(frame.Header, frame.Command, ErrCommandFormatError)
	}

	// エリアコードからエリアIDを取得
	areaID := AreaCodeToID(req.AreaCode)
	if areaID == "" {
		log.Printf("FINS: Unknown area code: 0x%02X", req.AreaCode)
		return BuildErrorResponse(frame.Header, frame.Command, ErrAreaClassError)
	}

	// データを読み込む
	data, err := h.store.ReadWords(areaID, uint32(req.Address), req.Count)
	if err != nil {
		log.Printf("FINS: Failed to read memory: %v", err)
		return BuildErrorResponse(frame.Header, frame.Command, ErrAddressRangeError)
	}

	log.Printf("FINS: Memory read: area=%s, addr=%d, count=%d", areaID, req.Address, req.Count)

	return BuildMemoryAreaReadResponse(frame.Header, frame.Command, EndCode{Main: ErrNormal}, data)
}

// handleMemoryAreaWrite はメモリエリア書き込みコマンドを処理する
func (h *Handler) handleMemoryAreaWrite(frame *Frame) []byte {
	req, err := ParseMemoryAreaWriteRequest(frame.Data)
	if err != nil {
		log.Printf("FINS: Failed to parse memory write request: %v", err)
		return BuildErrorResponse(frame.Header, frame.Command, ErrCommandFormatError)
	}

	// エリアコードからエリアIDを取得
	areaID := AreaCodeToID(req.AreaCode)
	if areaID == "" {
		log.Printf("FINS: Unknown area code: 0x%02X", req.AreaCode)
		return BuildErrorResponse(frame.Header, frame.Command, ErrAreaClassError)
	}

	// バイト列をワード列に変換（ビッグエンディアン）
	words := make([]uint16, req.Count)
	for i := 0; i < int(req.Count); i++ {
		words[i] = binary.BigEndian.Uint16(req.Data[i*2 : i*2+2])
	}

	// データを書き込む
	err = h.store.WriteWords(areaID, uint32(req.Address), words)
	if err != nil {
		log.Printf("FINS: Failed to write memory: %v", err)
		return BuildErrorResponse(frame.Header, frame.Command, ErrAddressRangeError)
	}

	log.Printf("FINS: Memory write: area=%s, addr=%d, count=%d", areaID, req.Address, req.Count)

	return BuildMemoryAreaWriteResponse(frame.Header, frame.Command, EndCode{Main: ErrNormal})
}

// HandleNodeAddressRequest はノードアドレス要求を処理する
func (h *Handler) HandleNodeAddressRequest(data []byte) []byte {
	req, err := ParseNodeAddressRequest(data)
	if err != nil {
		log.Printf("FINS: Failed to parse node address request: %v", err)
		return nil
	}

	log.Printf("FINS: Node address request from client node: %d, assigning server node: %d", req.ClientNode, h.nodeAddr)

	return BuildNodeAddressResponse(req.ClientNode, h.nodeAddr)
}

// SetNodeAddress はノードアドレスを設定する
func (h *Handler) SetNodeAddress(nodeAddr byte) {
	h.nodeAddr = nodeAddr
}

// SetNetworkID はネットワークIDを設定する
func (h *Handler) SetNetworkID(networkID byte) {
	h.networkID = networkID
}

// HandleUDPCommand はFINS/UDPコマンドを処理し、レスポンスを返す
func (h *Handler) HandleUDPCommand(frame *UDPFrame) []byte {
	if frame.Header == nil {
		return nil
	}

	cmdCode := frame.Command.Code()

	switch cmdCode {
	case CmdMemoryAreaRead:
		return h.handleUDPMemoryAreaRead(frame)
	case CmdMemoryAreaWrite:
		return h.handleUDPMemoryAreaWrite(frame)
	default:
		log.Printf("FINS/UDP: Unknown command: 0x%04X", cmdCode)
		return BuildUDPErrorResponse(frame.Header, frame.Command, ErrCommandNotSupported)
	}
}

// handleUDPMemoryAreaRead はUDP用メモリエリア読み込みコマンドを処理する
func (h *Handler) handleUDPMemoryAreaRead(frame *UDPFrame) []byte {
	req, err := ParseMemoryAreaReadRequest(frame.Data)
	if err != nil {
		log.Printf("FINS/UDP: Failed to parse memory read request: %v", err)
		return BuildUDPErrorResponse(frame.Header, frame.Command, ErrCommandFormatError)
	}

	// エリアコードからエリアIDを取得
	areaID := AreaCodeToID(req.AreaCode)
	if areaID == "" {
		log.Printf("FINS/UDP: Unknown area code: 0x%02X", req.AreaCode)
		return BuildUDPErrorResponse(frame.Header, frame.Command, ErrAreaClassError)
	}

	// データを読み込む
	data, err := h.store.ReadWords(areaID, uint32(req.Address), req.Count)
	if err != nil {
		log.Printf("FINS/UDP: Failed to read memory: %v", err)
		return BuildUDPErrorResponse(frame.Header, frame.Command, ErrAddressRangeError)
	}

	log.Printf("FINS/UDP: Memory read: area=%s, addr=%d, count=%d", areaID, req.Address, req.Count)

	return BuildUDPMemoryAreaReadResponse(frame.Header, frame.Command, EndCode{Main: ErrNormal}, data)
}

// handleUDPMemoryAreaWrite はUDP用メモリエリア書き込みコマンドを処理する
func (h *Handler) handleUDPMemoryAreaWrite(frame *UDPFrame) []byte {
	req, err := ParseMemoryAreaWriteRequest(frame.Data)
	if err != nil {
		log.Printf("FINS/UDP: Failed to parse memory write request: %v", err)
		return BuildUDPErrorResponse(frame.Header, frame.Command, ErrCommandFormatError)
	}

	// エリアコードからエリアIDを取得
	areaID := AreaCodeToID(req.AreaCode)
	if areaID == "" {
		log.Printf("FINS/UDP: Unknown area code: 0x%02X", req.AreaCode)
		return BuildUDPErrorResponse(frame.Header, frame.Command, ErrAreaClassError)
	}

	// バイト列をワード列に変換（ビッグエンディアン）
	words := make([]uint16, req.Count)
	for i := 0; i < int(req.Count); i++ {
		words[i] = binary.BigEndian.Uint16(req.Data[i*2 : i*2+2])
	}

	// データを書き込む
	err = h.store.WriteWords(areaID, uint32(req.Address), words)
	if err != nil {
		log.Printf("FINS/UDP: Failed to write memory: %v", err)
		return BuildUDPErrorResponse(frame.Header, frame.Command, ErrAddressRangeError)
	}

	log.Printf("FINS/UDP: Memory write: area=%s, addr=%d, count=%d", areaID, req.Address, req.Count)

	return BuildUDPMemoryAreaWriteResponse(frame.Header, frame.Command, EndCode{Main: ErrNormal})
}
