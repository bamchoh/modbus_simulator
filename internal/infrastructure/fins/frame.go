package fins

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// FINS/TCP Header Constants
const (
	FINSTCPHeaderSize = 16
	FINSMagic         = "FINS"
)

// FINS/TCP Command codes
const (
	TCPCmdNOOP             uint32 = 0 // No operation
	TCPCmdNodeAddressSend  uint32 = 1 // Client to server node address data send
	TCPCmdNodeAddressRecv  uint32 = 2 // Server to client node address data send
	TCPCmdFrameSend        uint32 = 2 // Frame send (same code as NodeAddressRecv)
	TCPCmdFrameSendError   uint32 = 3 // Frame send error notification
)

// FINS Command codes
const (
	CmdMemoryAreaRead  uint16 = 0x0101 // Memory Area Read
	CmdMemoryAreaWrite uint16 = 0x0102 // Memory Area Write
)

// TCPHeader はFINS/TCPヘッダー
type TCPHeader struct {
	Magic   [4]byte // "FINS"
	Length  uint32  // データ長（ヘッダー以降）
	Command uint32  // コマンド
	Error   uint32  // エラーコード
}

// ParseTCPHeader はバイト列からTCPヘッダーをパースする
func ParseTCPHeader(data []byte) (*TCPHeader, error) {
	if len(data) < FINSTCPHeaderSize {
		return nil, errors.New("data too short for FINS/TCP header")
	}

	header := &TCPHeader{}
	copy(header.Magic[:], data[0:4])

	if string(header.Magic[:]) != FINSMagic {
		return nil, fmt.Errorf("invalid FINS magic: expected %s, got %s", FINSMagic, string(header.Magic[:]))
	}

	header.Length = binary.BigEndian.Uint32(data[4:8])
	header.Command = binary.BigEndian.Uint32(data[8:12])
	header.Error = binary.BigEndian.Uint32(data[12:16])

	return header, nil
}

// Bytes はTCPヘッダーをバイト列に変換する
func (h *TCPHeader) Bytes() []byte {
	buf := make([]byte, FINSTCPHeaderSize)
	copy(buf[0:4], FINSMagic)
	binary.BigEndian.PutUint32(buf[4:8], h.Length)
	binary.BigEndian.PutUint32(buf[8:12], h.Command)
	binary.BigEndian.PutUint32(buf[12:16], h.Error)
	return buf
}

// CommandHeader はFINSコマンドヘッダー（10バイト）
type CommandHeader struct {
	ICF byte // Information Control Field
	RSV byte // Reserved (must be 0)
	GCT byte // Gateway Count (0-7)
	DNA byte // Destination Network Address
	DA1 byte // Destination Node Address
	DA2 byte // Destination Unit Address
	SNA byte // Source Network Address
	SA1 byte // Source Node Address
	SA2 byte // Source Unit Address
	SID byte // Service ID
}

const CommandHeaderSize = 10

// ParseCommandHeader はバイト列からコマンドヘッダーをパースする
func ParseCommandHeader(data []byte) (*CommandHeader, error) {
	if len(data) < CommandHeaderSize {
		return nil, errors.New("data too short for FINS command header")
	}

	return &CommandHeader{
		ICF: data[0],
		RSV: data[1],
		GCT: data[2],
		DNA: data[3],
		DA1: data[4],
		DA2: data[5],
		SNA: data[6],
		SA1: data[7],
		SA2: data[8],
		SID: data[9],
	}, nil
}

// Bytes はコマンドヘッダーをバイト列に変換する
func (h *CommandHeader) Bytes() []byte {
	return []byte{
		h.ICF, h.RSV, h.GCT,
		h.DNA, h.DA1, h.DA2,
		h.SNA, h.SA1, h.SA2,
		h.SID,
	}
}

// IsRequest はリクエストかどうかを返す
func (h *CommandHeader) IsRequest() bool {
	return (h.ICF & 0x40) == 0
}

// NeedsResponse はレスポンスが必要かどうかを返す
func (h *CommandHeader) NeedsResponse() bool {
	return (h.ICF & 0x01) == 0
}

// CreateResponseHeader はレスポンス用のヘッダーを作成する
func (h *CommandHeader) CreateResponseHeader() *CommandHeader {
	return &CommandHeader{
		ICF: h.ICF | 0x40, // Set response bit
		RSV: h.RSV,
		GCT: h.GCT,
		DNA: h.SNA, // Swap source and destination
		DA1: h.SA1,
		DA2: h.SA2,
		SNA: h.DNA,
		SA1: h.DA1,
		SA2: h.DA2,
		SID: h.SID,
	}
}

// Command はFINSコマンドコード（MRC + SRC）
type Command struct {
	MRC byte // Main Request Code
	SRC byte // Sub Request Code
}

// Code はコマンドコードを返す
func (c Command) Code() uint16 {
	return (uint16(c.MRC) << 8) | uint16(c.SRC)
}

// ParseCommand はバイト列からコマンドコードをパースする
func ParseCommand(data []byte) (Command, error) {
	if len(data) < 2 {
		return Command{}, errors.New("data too short for command code")
	}
	return Command{MRC: data[0], SRC: data[1]}, nil
}

// Bytes はコマンドコードをバイト列に変換する
func (c Command) Bytes() []byte {
	return []byte{c.MRC, c.SRC}
}

// Frame はFINSフレーム全体
type Frame struct {
	TCPHeader *TCPHeader
	Header    *CommandHeader
	Command   Command
	Data      []byte
}

// ParseFrame はバイト列からフレーム全体をパースする
func ParseFrame(data []byte) (*Frame, error) {
	if len(data) < FINSTCPHeaderSize {
		return nil, errors.New("data too short for FINS frame")
	}

	tcpHeader, err := ParseTCPHeader(data[:FINSTCPHeaderSize])
	if err != nil {
		return nil, err
	}

	// ノードアドレス送信コマンドの場合
	if tcpHeader.Command == TCPCmdNodeAddressSend {
		frame := &Frame{
			TCPHeader: tcpHeader,
		}
		if len(data) > FINSTCPHeaderSize {
			frame.Data = data[FINSTCPHeaderSize:]
		}
		return frame, nil
	}

	// 通常のFINSコマンドフレーム
	if len(data) < FINSTCPHeaderSize+CommandHeaderSize+2 {
		return nil, errors.New("data too short for FINS command frame")
	}

	cmdHeader, err := ParseCommandHeader(data[FINSTCPHeaderSize : FINSTCPHeaderSize+CommandHeaderSize])
	if err != nil {
		return nil, err
	}

	cmd, err := ParseCommand(data[FINSTCPHeaderSize+CommandHeaderSize : FINSTCPHeaderSize+CommandHeaderSize+2])
	if err != nil {
		return nil, err
	}

	frame := &Frame{
		TCPHeader: tcpHeader,
		Header:    cmdHeader,
		Command:   cmd,
	}

	if len(data) > FINSTCPHeaderSize+CommandHeaderSize+2 {
		frame.Data = data[FINSTCPHeaderSize+CommandHeaderSize+2:]
	}

	return frame, nil
}

// Bytes はフレーム全体をバイト列に変換する
func (f *Frame) Bytes() []byte {
	var payload []byte

	if f.Header != nil {
		payload = append(payload, f.Header.Bytes()...)
		payload = append(payload, f.Command.Bytes()...)
	}

	if f.Data != nil {
		payload = append(payload, f.Data...)
	}

	// TCPヘッダーの長さを更新
	f.TCPHeader.Length = uint32(len(payload))

	result := f.TCPHeader.Bytes()
	result = append(result, payload...)

	return result
}

// NodeAddressRequest はノードアドレス要求
type NodeAddressRequest struct {
	ClientNode byte
}

// ParseNodeAddressRequest はノードアドレス要求をパースする
func ParseNodeAddressRequest(data []byte) (*NodeAddressRequest, error) {
	if len(data) < 4 {
		return nil, errors.New("data too short for node address request")
	}
	// 実際には4バイトだが、クライアントノードは最初の4バイト目（0ベース）にある
	return &NodeAddressRequest{
		ClientNode: data[3],
	}, nil
}

// NodeAddressResponse はノードアドレス応答
type NodeAddressResponse struct {
	ClientNode byte
	ServerNode byte
}

// Bytes はノードアドレス応答をバイト列に変換する
func (r *NodeAddressResponse) Bytes() []byte {
	return []byte{0x00, 0x00, 0x00, r.ClientNode, 0x00, 0x00, 0x00, r.ServerNode}
}

// MemoryAreaReadRequest はメモリエリア読み込みリクエスト
type MemoryAreaReadRequest struct {
	AreaCode   MemoryAreaCode
	Address    uint16 // ワードアドレス
	BitAddress byte   // ビットアドレス（ワードアクセス時は0）
	Count      uint16 // 読み込むワード数
}

// ParseMemoryAreaReadRequest はメモリエリア読み込みリクエストをパースする
func ParseMemoryAreaReadRequest(data []byte) (*MemoryAreaReadRequest, error) {
	if len(data) < 6 {
		return nil, errors.New("data too short for memory area read request")
	}

	return &MemoryAreaReadRequest{
		AreaCode:   MemoryAreaCode(data[0]),
		Address:    binary.BigEndian.Uint16(data[1:3]),
		BitAddress: data[3],
		Count:      binary.BigEndian.Uint16(data[4:6]),
	}, nil
}

// MemoryAreaWriteRequest はメモリエリア書き込みリクエスト
type MemoryAreaWriteRequest struct {
	AreaCode   MemoryAreaCode
	Address    uint16 // ワードアドレス
	BitAddress byte   // ビットアドレス（ワードアクセス時は0）
	Count      uint16 // 書き込むワード数
	Data       []byte // 書き込むデータ
}

// ParseMemoryAreaWriteRequest はメモリエリア書き込みリクエストをパースする
func ParseMemoryAreaWriteRequest(data []byte) (*MemoryAreaWriteRequest, error) {
	if len(data) < 6 {
		return nil, errors.New("data too short for memory area write request")
	}

	count := binary.BigEndian.Uint16(data[4:6])
	expectedDataLen := int(count) * 2

	if len(data) < 6+expectedDataLen {
		return nil, fmt.Errorf("data too short: expected %d bytes, got %d", 6+expectedDataLen, len(data))
	}

	return &MemoryAreaWriteRequest{
		AreaCode:   MemoryAreaCode(data[0]),
		Address:    binary.BigEndian.Uint16(data[1:3]),
		BitAddress: data[3],
		Count:      count,
		Data:       data[6 : 6+expectedDataLen],
	}, nil
}

// BuildMemoryAreaReadResponse はメモリエリア読み込みレスポンスを構築する
func BuildMemoryAreaReadResponse(cmdHeader *CommandHeader, cmd Command, endCode EndCode, data []uint16) []byte {
	respHeader := cmdHeader.CreateResponseHeader()

	// レスポンスデータを構築
	var respData []byte
	respData = append(respData, endCode.Bytes()...)

	// ワードデータをビッグエンディアンで追加
	for _, word := range data {
		respData = append(respData, byte(word>>8), byte(word&0xFF))
	}

	// フレームを構築
	frame := &Frame{
		TCPHeader: &TCPHeader{
			Command: TCPCmdFrameSend,
			Error:   0,
		},
		Header:  respHeader,
		Command: cmd,
		Data:    respData,
	}

	return frame.Bytes()
}

// BuildMemoryAreaWriteResponse はメモリエリア書き込みレスポンスを構築する
func BuildMemoryAreaWriteResponse(cmdHeader *CommandHeader, cmd Command, endCode EndCode) []byte {
	respHeader := cmdHeader.CreateResponseHeader()

	// フレームを構築
	frame := &Frame{
		TCPHeader: &TCPHeader{
			Command: TCPCmdFrameSend,
			Error:   0,
		},
		Header:  respHeader,
		Command: cmd,
		Data:    endCode.Bytes(),
	}

	return frame.Bytes()
}

// BuildNodeAddressResponse はノードアドレス応答フレームを構築する
func BuildNodeAddressResponse(clientNode, serverNode byte) []byte {
	resp := &NodeAddressResponse{
		ClientNode: clientNode,
		ServerNode: serverNode,
	}

	tcpHeader := &TCPHeader{
		Command: TCPCmdNodeAddressRecv,
		Error:   0,
	}

	data := resp.Bytes()
	tcpHeader.Length = uint32(len(data))

	result := tcpHeader.Bytes()
	result = append(result, data...)

	return result
}

// BuildErrorResponse はエラーレスポンスを構築する
func BuildErrorResponse(cmdHeader *CommandHeader, cmd Command, finsError FINSError) []byte {
	return BuildMemoryAreaReadResponse(cmdHeader, cmd, EndCode{Main: finsError}, nil)
}
