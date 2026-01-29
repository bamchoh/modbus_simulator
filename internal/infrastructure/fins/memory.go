package fins

// MemoryAreaCode はFINSメモリエリアコード
type MemoryAreaCode byte

const (
	// ワードアクセス用メモリエリアコード
	AreaCIO MemoryAreaCode = 0x30 // CIO (Core I/O)
	AreaWR  MemoryAreaCode = 0x31 // Work Area
	AreaHR  MemoryAreaCode = 0xB0 // Holding Area
	AreaAR  MemoryAreaCode = 0xB1 // Auxiliary Area
	AreaDM  MemoryAreaCode = 0x82 // Data Memory
	AreaTIM MemoryAreaCode = 0x09 // Timer PV
	AreaCNT MemoryAreaCode = 0x09 // Counter PV (same code as TIM, distinguished by address)

	// ビットアクセス用メモリエリアコード
	AreaCIOBit MemoryAreaCode = 0x30 // CIO Bit
	AreaWRBit  MemoryAreaCode = 0x31 // Work Area Bit
	AreaHRBit  MemoryAreaCode = 0xB0 // Holding Area Bit
	AreaARBit  MemoryAreaCode = 0xB1 // Auxiliary Area Bit
)

// MemoryAreaInfo はメモリエリアの情報
type MemoryAreaInfo struct {
	Code        MemoryAreaCode
	ID          string
	DisplayName string
	Size        uint32 // ワード数
	IsBit       bool
	ReadOnly    bool
}

// 標準メモリエリア定義
var StandardMemoryAreas = []MemoryAreaInfo{
	{Code: AreaCIO, ID: "CIO", DisplayName: "CIO Area", Size: 6144, IsBit: false, ReadOnly: false},
	{Code: AreaWR, ID: "WR", DisplayName: "Work Area", Size: 512, IsBit: false, ReadOnly: false},
	{Code: AreaHR, ID: "HR", DisplayName: "Holding Area", Size: 1536, IsBit: false, ReadOnly: false},
	{Code: AreaAR, ID: "AR", DisplayName: "Auxiliary Area", Size: 960, IsBit: false, ReadOnly: false},
	{Code: AreaDM, ID: "DM", DisplayName: "Data Memory", Size: 32768, IsBit: false, ReadOnly: false},
	{Code: AreaTIM, ID: "TIM", DisplayName: "Timer", Size: 4096, IsBit: false, ReadOnly: false},
	{Code: AreaCNT, ID: "CNT", DisplayName: "Counter", Size: 4096, IsBit: false, ReadOnly: false},
}

// AreaCodeToID はエリアコードからエリアIDを取得する
func AreaCodeToID(code MemoryAreaCode) string {
	switch code {
	case AreaCIO:
		return "CIO"
	case AreaWR:
		return "WR"
	case AreaHR:
		return "HR"
	case AreaAR:
		return "AR"
	case AreaDM:
		return "DM"
	case AreaTIM:
		return "TIM"
	default:
		return ""
	}
}

// IDToAreaCode はエリアIDからエリアコードを取得する
func IDToAreaCode(id string) (MemoryAreaCode, bool) {
	switch id {
	case "CIO":
		return AreaCIO, true
	case "WR":
		return AreaWR, true
	case "HR":
		return AreaHR, true
	case "AR":
		return AreaAR, true
	case "DM":
		return AreaDM, true
	case "TIM":
		return AreaTIM, true
	case "CNT":
		return AreaCNT, true
	default:
		return 0, false
	}
}

// GetAreaInfo はエリアIDからエリア情報を取得する
func GetAreaInfo(id string) (MemoryAreaInfo, bool) {
	for _, area := range StandardMemoryAreas {
		if area.ID == id {
			return area, true
		}
	}
	return MemoryAreaInfo{}, false
}

// GetAreaInfoByCode はエリアコードからエリア情報を取得する
func GetAreaInfoByCode(code MemoryAreaCode) (MemoryAreaInfo, bool) {
	for _, area := range StandardMemoryAreas {
		if area.Code == code {
			return area, true
		}
	}
	return MemoryAreaInfo{}, false
}
