package rtu

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// ASCII フレーム定数
const (
	ASCIIFrameStart = ':' // 0x3A
	ASCIIFrameCR    = '\r' // 0x0D
	ASCIIFrameLF    = '\n' // 0x0A
)

// ParseASCIIFrame はASCIIフレームを解析してバイナリデータを返す
// 入力: ":AABBCCDD...LRC\r\n" 形式の文字列
// 出力: バイナリデータ（LRC検証済み）
func ParseASCIIFrame(frame []byte) ([]byte, error) {
	// 最小長チェック: ':' + UnitID(2) + FC(2) + LRC(2) + CR + LF = 9
	if len(frame) < 9 {
		return nil, ErrFrameTooShort
	}

	// 開始文字チェック
	if frame[0] != ASCIIFrameStart {
		return nil, fmt.Errorf("invalid start character: expected ':', got '%c'", frame[0])
	}

	// 終了文字チェック
	if frame[len(frame)-2] != ASCIIFrameCR || frame[len(frame)-1] != ASCIIFrameLF {
		return nil, fmt.Errorf("invalid end characters: expected CR LF")
	}

	// HEX部分を抽出（':'とCR LFを除く）
	hexStr := string(frame[1 : len(frame)-2])

	// HEX文字列の長さは偶数でなければならない
	if len(hexStr)%2 != 0 {
		return nil, fmt.Errorf("invalid hex string length")
	}

	// HEX文字列をバイナリに変換
	data, err := hex.DecodeString(strings.ToUpper(hexStr))
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %w", err)
	}

	// 最低限のデータ長チェック（UnitID + FC + LRC = 3バイト）
	if len(data) < 3 {
		return nil, ErrFrameTooShort
	}

	// LRC検証
	dataWithoutLRC := data[:len(data)-1]
	receivedLRC := data[len(data)-1]
	if !CheckLRC(dataWithoutLRC, receivedLRC) {
		return nil, fmt.Errorf("LRC check failed")
	}

	return dataWithoutLRC, nil
}

// ParseASCIIRequest はASCIIフレームからリクエストを解析する
func ParseASCIIRequest(frame []byte) (*Request, error) {
	// ASCIIフレームをバイナリに変換
	data, err := ParseASCIIFrame(frame)
	if err != nil {
		return nil, err
	}

	// 最小データ長チェック（UnitID + FC + データ2バイト = 4バイト）
	if len(data) < 4 {
		return nil, ErrFrameTooShort
	}

	req := &Request{
		UnitID:       data[0],
		FunctionCode: data[1],
	}

	switch req.FunctionCode {
	case FuncReadCoils, FuncReadDiscreteInputs, FuncReadHoldingRegisters, FuncReadInputRegisters:
		// 読み取りリクエスト: Address(2) + Quantity(2)
		if len(data) < 6 {
			return nil, ErrFrameTooShort
		}
		req.Address = binary.BigEndian.Uint16(data[2:4])
		req.Quantity = binary.BigEndian.Uint16(data[4:6])

	case FuncWriteSingleCoil, FuncWriteSingleRegister:
		// 単一書き込み: Address(2) + Value(2)
		if len(data) < 6 {
			return nil, ErrFrameTooShort
		}
		req.Address = binary.BigEndian.Uint16(data[2:4])
		req.Quantity = 1
		req.Data = data[4:6]

	case FuncWriteMultipleCoils:
		// 複数コイル書き込み: Address(2) + Quantity(2) + ByteCount(1) + Data(N)
		if len(data) < 7 {
			return nil, ErrFrameTooShort
		}
		req.Address = binary.BigEndian.Uint16(data[2:4])
		req.Quantity = binary.BigEndian.Uint16(data[4:6])
		byteCount := int(data[6])
		if len(data) < 7+byteCount {
			return nil, ErrFrameTooShort
		}
		req.Data = data[7 : 7+byteCount]

	case FuncWriteMultipleRegisters:
		// 複数レジスタ書き込み: Address(2) + Quantity(2) + ByteCount(1) + Data(N)
		if len(data) < 7 {
			return nil, ErrFrameTooShort
		}
		req.Address = binary.BigEndian.Uint16(data[2:4])
		req.Quantity = binary.BigEndian.Uint16(data[4:6])
		byteCount := int(data[6])
		if len(data) < 7+byteCount {
			return nil, ErrFrameTooShort
		}
		req.Data = data[7 : 7+byteCount]

	default:
		return nil, fmt.Errorf("unsupported function code: 0x%02X", req.FunctionCode)
	}

	return req, nil
}

// BuildASCIIFrame はバイナリデータからASCIIフレームを構築する
func BuildASCIIFrame(data []byte) []byte {
	// LRCを計算
	lrc := LRC(data)
	dataWithLRC := append(data, lrc)

	// HEX文字列に変換
	hexStr := strings.ToUpper(hex.EncodeToString(dataWithLRC))

	// フレームを構築: ':' + HEX + CR + LF
	frame := make([]byte, 1+len(hexStr)+2)
	frame[0] = ASCIIFrameStart
	copy(frame[1:], hexStr)
	frame[len(frame)-2] = ASCIIFrameCR
	frame[len(frame)-1] = ASCIIFrameLF

	return frame
}

// BuildASCIIReadBitsResponse は読み取りレスポンスを構築する（コイル/ディスクリート入力用）
func BuildASCIIReadBitsResponse(unitID, funcCode byte, values []bool) []byte {
	byteCount := (len(values) + 7) / 8
	data := make([]byte, 3+byteCount)
	data[0] = unitID
	data[1] = funcCode
	data[2] = byte(byteCount)

	// ビットをバイトにパック
	for i, v := range values {
		if v {
			byteIdx := i / 8
			bitIdx := uint(i % 8)
			data[3+byteIdx] |= 1 << bitIdx
		}
	}

	return BuildASCIIFrame(data)
}

// BuildASCIIReadRegistersResponse は読み取りレスポンスを構築する（レジスタ用）
func BuildASCIIReadRegistersResponse(unitID, funcCode byte, values []uint16) []byte {
	byteCount := len(values) * 2
	data := make([]byte, 3+byteCount)
	data[0] = unitID
	data[1] = funcCode
	data[2] = byte(byteCount)

	for i, v := range values {
		binary.BigEndian.PutUint16(data[3+i*2:], v)
	}

	return BuildASCIIFrame(data)
}

// BuildASCIIWriteSingleResponse は単一書き込みレスポンスを構築する
func BuildASCIIWriteSingleResponse(unitID, funcCode byte, address uint16, value uint16) []byte {
	data := make([]byte, 6)
	data[0] = unitID
	data[1] = funcCode
	binary.BigEndian.PutUint16(data[2:4], address)
	binary.BigEndian.PutUint16(data[4:6], value)

	return BuildASCIIFrame(data)
}

// BuildASCIIWriteMultipleResponse は複数書き込みレスポンスを構築する
func BuildASCIIWriteMultipleResponse(unitID, funcCode byte, address, quantity uint16) []byte {
	data := make([]byte, 6)
	data[0] = unitID
	data[1] = funcCode
	binary.BigEndian.PutUint16(data[2:4], address)
	binary.BigEndian.PutUint16(data[4:6], quantity)

	return BuildASCIIFrame(data)
}

// BuildASCIIExceptionResponse は例外レスポンスを構築する
func BuildASCIIExceptionResponse(unitID, funcCode, exceptionCode byte) []byte {
	data := make([]byte, 3)
	data[0] = unitID
	data[1] = funcCode | 0x80 // 例外フラグ
	data[2] = exceptionCode

	return BuildASCIIFrame(data)
}
