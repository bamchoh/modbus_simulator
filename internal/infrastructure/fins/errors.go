package fins

import "fmt"

// FINSエラーコード
type FINSError uint16

const (
	// 正常終了
	ErrNormal FINSError = 0x0000

	// ローカルノードエラー
	ErrLocalNodeNotInNetwork  FINSError = 0x0101
	ErrTokenTimeout           FINSError = 0x0102
	ErrRetryFailed            FINSError = 0x0103
	ErrTooManySendFrames      FINSError = 0x0104
	ErrNodeAddressRangeError  FINSError = 0x0105
	ErrNodeAddressDuplication FINSError = 0x0106

	// 宛先ノードエラー
	ErrDestNotInNetwork       FINSError = 0x0201
	ErrUnitMissing            FINSError = 0x0202
	ErrThirdNodeMissing       FINSError = 0x0203
	ErrDestNodeBusy           FINSError = 0x0204
	ErrResponseTimeout        FINSError = 0x0205

	// 通信コントローラエラー
	ErrCommControllerError    FINSError = 0x0301
	ErrCPUUnitError           FINSError = 0x0302
	ErrControllerBoardError   FINSError = 0x0303
	ErrUnitNumberError        FINSError = 0x0304
	ErrBroadcastNotSupported  FINSError = 0x0305

	// サービス非対応エラー
	ErrCommandNotSupported    FINSError = 0x0401
	ErrModelNotSupported      FINSError = 0x0402

	// ルーティングエラー
	ErrRoutingTableError      FINSError = 0x0501
	ErrRelayTableError        FINSError = 0x0502
	ErrSendBufferFull         FINSError = 0x0503
	ErrSendBusy               FINSError = 0x0504

	// コマンド形式エラー
	ErrCommandTooLong         FINSError = 0x1001
	ErrCommandTooShort        FINSError = 0x1002
	ErrItemsDataMismatch      FINSError = 0x1003
	ErrCommandFormatError     FINSError = 0x1004
	ErrHeaderError            FINSError = 0x1005

	// パラメータエラー
	ErrAreaClassError         FINSError = 0x1101
	ErrAccessSizeError        FINSError = 0x1102
	ErrAddressRangeError      FINSError = 0x1103
	ErrAddressRangeExceeded   FINSError = 0x1104
	ErrProgramMissing         FINSError = 0x1106

	// 読み出しエラー
	ErrReadNotPossible        FINSError = 0x2002
	ErrWriteNotPossible       FINSError = 0x2003
	ErrCannotExecuteInRun     FINSError = 0x2004
	ErrCannotExecuteInPgm     FINSError = 0x2005
)

// EndCode はFINSレスポンスの終了コード
type EndCode struct {
	Main FINSError
	Sub  uint8
}

// IsSuccess は正常終了かどうかを返す
func (e EndCode) IsSuccess() bool {
	return e.Main == ErrNormal
}

// Bytes はエンドコードをバイト列に変換する
func (e EndCode) Bytes() []byte {
	return []byte{byte(e.Main >> 8), byte(e.Main & 0xFF)}
}

// Error はエラーメッセージを返す
func (e FINSError) Error() string {
	switch e {
	case ErrNormal:
		return "Normal completion"
	case ErrLocalNodeNotInNetwork:
		return "Local node not in network"
	case ErrTokenTimeout:
		return "Token timeout"
	case ErrDestNotInNetwork:
		return "Destination node not in network"
	case ErrResponseTimeout:
		return "Response timeout"
	case ErrCommandNotSupported:
		return "Command not supported"
	case ErrCommandTooLong:
		return "Command too long"
	case ErrCommandTooShort:
		return "Command too short"
	case ErrAreaClassError:
		return "Area classification error"
	case ErrAccessSizeError:
		return "Access size error"
	case ErrAddressRangeError:
		return "Address range error"
	case ErrAddressRangeExceeded:
		return "Address range exceeded"
	case ErrReadNotPossible:
		return "Read not possible"
	case ErrWriteNotPossible:
		return "Write not possible"
	default:
		return fmt.Sprintf("FINS error: 0x%04X", uint16(e))
	}
}
