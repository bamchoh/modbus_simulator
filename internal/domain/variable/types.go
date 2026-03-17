package variable

import (
	"fmt"
	"strconv"
	"strings"
)

// DataType はIEC 61131-3データ型
type DataType string

const (
	TypeBOOL          DataType = "BOOL"          // ブール値
	TypeSINT          DataType = "SINT"          // 符号付き8ビット整数
	TypeINT           DataType = "INT"           // 符号付き16ビット整数
	TypeDINT          DataType = "DINT"          // 符号付き32ビット整数
	TypeLINT          DataType = "LINT"          // 符号付き64ビット整数
	TypeUSINT         DataType = "USINT"         // 符号なし8ビット整数
	TypeUINT          DataType = "UINT"          // 符号なし16ビット整数
	TypeUDINT         DataType = "UDINT"         // 符号なし32ビット整数
	TypeULINT         DataType = "ULINT"         // 符号なし64ビット整数
	TypeREAL          DataType = "REAL"          // 32ビット浮動小数点
	TypeLREAL         DataType = "LREAL"         // 64ビット浮動小数点
	TypeSTRING        DataType = "STRING"        // 文字列
	TypeTIME          DataType = "TIME"          // 時間間隔（ミリ秒）
	TypeDATE          DataType = "DATE"          // 日付（1970-01-01からの日数、uint32で2ワード保存）
	TypeTIME_OF_DAY   DataType = "TIME_OF_DAY"   // 1日の中の時刻（ミリ秒）
	TypeDATE_AND_TIME DataType = "DATE_AND_TIME" // 日付と時刻（1970-01-01 00:00:00からの秒数、uint32で2ワード保存）
)

// AllDataTypes はすべてのデータ型を返す
func AllDataTypes() []DataType {
	return []DataType{
		TypeBOOL, TypeSINT, TypeINT, TypeDINT, TypeLINT,
		TypeUSINT, TypeUINT, TypeUDINT, TypeULINT,
		TypeREAL, TypeLREAL, TypeSTRING,
		TypeTIME, TypeDATE, TypeTIME_OF_DAY, TypeDATE_AND_TIME,
	}
}

// WordCount はデータ型が占有するワード数を返す
func (dt DataType) WordCount() int {
	switch dt {
	case TypeBOOL, TypeSINT, TypeUSINT, TypeINT, TypeUINT:
		return 1
	case TypeDINT, TypeUDINT, TypeREAL, TypeTIME, TypeTIME_OF_DAY:
		return 2
	case TypeLREAL, TypeLINT, TypeULINT, TypeDATE, TypeDATE_AND_TIME:
		return 4
	case TypeSTRING:
		return 1 // 後方互換: 長さ未指定のSTRING
	default:
		// STRING[n] の場合
		if dt.IsStringType() {
			maxLen, err := ParseStringType(dt)
			if err == nil {
				return (maxLen + 1) / 2 // ceil(n / 2)
			}
		}
		return 1
	}
}

// IsBitType はビット型かどうかを返す
func (dt DataType) IsBitType() bool {
	return dt == TypeBOOL
}

// IsStringType は固定長文字列型 STRING[n] かどうかを判定する
func (dt DataType) IsStringType() bool {
	s := string(dt)
	if !strings.HasPrefix(s, "STRING[") || !strings.HasSuffix(s, "]") {
		return false
	}
	inner := s[7 : len(s)-1]
	_, err := strconv.Atoi(inner)
	return err == nil
}

// ParseStringType は STRING[n] から最大バイト長 n を取得する
func ParseStringType(dt DataType) (maxLen int, err error) {
	s := string(dt)
	if !strings.HasPrefix(s, "STRING[") || !strings.HasSuffix(s, "]") {
		return 0, fmt.Errorf("not a string type: %s", dt)
	}
	inner := s[7 : len(s)-1]
	maxLen, err = strconv.Atoi(inner)
	if err != nil || maxLen <= 0 {
		return 0, fmt.Errorf("invalid string length: %s", inner)
	}
	return maxLen, nil
}

// NewStringType は固定長文字列型の DataType を生成する
func NewStringType(maxLen int) DataType {
	return DataType(fmt.Sprintf("STRING[%d]", maxLen))
}

// DefaultValue はデータ型のデフォルト値を返す
func (dt DataType) DefaultValue() interface{} {
	switch dt {
	case TypeBOOL:
		return false
	case TypeSINT:
		return int8(0)
	case TypeINT:
		return int16(0)
	case TypeDINT:
		return int32(0)
	case TypeUSINT:
		return uint8(0)
	case TypeUINT:
		return uint16(0)
	case TypeUDINT:
		return uint32(0)
	case TypeLINT:
		return int64(0)
	case TypeULINT:
		return uint64(0)
	case TypeREAL:
		return float32(0)
	case TypeLREAL:
		return float64(0)
	case TypeSTRING:
		return ""
	case TypeTIME:
		return "T#0ms"
	case TypeDATE:
		return "D#1970-01-01"
	case TypeTIME_OF_DAY:
		return "TOD#00:00:00"
	case TypeDATE_AND_TIME:
		return "DT#1970-01-01-00:00:00"
	default:
		if dt.IsStringType() {
			return ""
		}
		return nil
	}
}

// IsValid はデータ型が有効かどうかを返す（スカラー型のみ）
func (dt DataType) IsValid() bool {
	for _, t := range AllDataTypes() {
		if dt == t {
			return true
		}
	}
	// STRING[n] も有効なスカラー型
	if dt.IsStringType() {
		return true
	}
	return false
}

// IsArrayType は配列型かどうかを判定する
// 例: "ARRAY[0..9] OF INT", "ARRAY[0..2, 0..4] OF INT"
// 後方互換: "ARRAY[INT;10]" 形式も認識する
func (dt DataType) IsArrayType() bool {
	s := string(dt)
	if !strings.HasPrefix(s, "ARRAY[") {
		return false
	}
	// IEC 61131-3 形式: "] OF " を含む
	if strings.Contains(s, "] OF ") {
		return true
	}
	// 旧形式 (後方互換): "ARRAY[" で始まり "]" で終わる
	return strings.HasSuffix(s, "]")
}

// IsStructType は構造体型かどうかを判定する（スカラーでも配列でもない場合）
func (dt DataType) IsStructType() bool {
	return !dt.IsValid() && !dt.IsArrayType() && string(dt) != ""
}

// ParseArrayLower は配列型文字列から下限値を返す。
// 例: "ARRAY[2..9] OF INT" → (2, true)
// 後方互換の旧形式 "ARRAY[INT;10]" → (0, true) （旧形式は常に下限0）
// 配列型でない場合は (0, false)
func ParseArrayLower(dt DataType) (lower int, isArray bool) {
	s := string(dt)
	if !strings.HasPrefix(s, "ARRAY[") {
		return 0, false
	}
	// IEC 61131-3 形式: "ARRAY[n..m, ...] OF elemType"
	ofIdx := strings.Index(s, "] OF ")
	if ofIdx >= 0 {
		dimsStr := s[6:ofIdx]
		dimParts := strings.Split(dimsStr, ",")
		firstDim := strings.TrimSpace(dimParts[0])
		parts := strings.SplitN(firstDim, "..", 2)
		if len(parts) != 2 {
			return 0, false
		}
		lo, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return 0, false
		}
		return lo, true
	}
	// 旧形式（後方互換）: lower は常に 0
	if strings.HasSuffix(s, "]") {
		return 0, true
	}
	return 0, false
}

// parseArrayDimension は "lower..upper" 形式の次元文字列からサイズを返す
// 例: "0..9" → 10
func parseArrayDimension(s string) (int, error) {
	parts := strings.SplitN(strings.TrimSpace(s), "..", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid dimension format %q (expected 'lower..upper')", s)
	}
	lower, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, fmt.Errorf("invalid lower bound %q: %w", parts[0], err)
	}
	upper, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, fmt.Errorf("invalid upper bound %q: %w", parts[1], err)
	}
	if upper < lower {
		return 0, fmt.Errorf("upper bound %d must be >= lower bound %d", upper, lower)
	}
	return upper - lower + 1, nil
}

// ParseArrayType は配列型から要素型とサイズを取得する
//
// IEC 61131-3 形式:
//   - "ARRAY[0..9] OF INT"        → (TypeINT, 10, nil)
//   - "ARRAY[0..2, 0..4] OF INT"  → ("ARRAY[0..4] OF INT", 3, nil)
//
// 後方互換（旧形式）:
//   - "ARRAY[INT;10]"             → (TypeINT, 10, nil)
func ParseArrayType(dt DataType) (elementType DataType, size int, err error) {
	s := string(dt)
	if !strings.HasPrefix(s, "ARRAY[") {
		return "", 0, fmt.Errorf("not an array type: %s", dt)
	}

	// IEC 61131-3 形式: "ARRAY[dims] OF elemType"
	ofIdx := strings.Index(s, "] OF ")
	if ofIdx >= 0 {
		dimsStr := s[6:ofIdx]  // "0..9" or "0..2, 0..4"
		elemStr := s[ofIdx+5:] // "INT" or "ARRAY[0..4] OF INT"

		dimParts := strings.Split(dimsStr, ",")
		size, err = parseArrayDimension(dimParts[0])
		if err != nil {
			return "", 0, fmt.Errorf("invalid array type %q: %w", dt, err)
		}

		if len(dimParts) == 1 {
			// 1次元: 要素型はそのまま
			elementType = DataType(elemStr)
		} else {
			// 多次元: 残りの次元で内側の配列型を再構築
			trimmed := make([]string, len(dimParts)-1)
			for i, d := range dimParts[1:] {
				trimmed[i] = strings.TrimSpace(d)
			}
			elementType = DataType(fmt.Sprintf("ARRAY[%s] OF %s", strings.Join(trimmed, ", "), elemStr))
		}
		if !elementType.IsValid() && !elementType.IsStructType() && !elementType.IsArrayType() {
			return "", 0, fmt.Errorf("invalid element type: %s", elemStr)
		}
		return elementType, size, nil
	}

	// 旧形式（後方互換）: "ARRAY[ElementType;Size]"
	if !strings.HasSuffix(s, "]") {
		return "", 0, fmt.Errorf("invalid array type: %s", dt)
	}
	inner := s[6 : len(s)-1] // "INT;10"
	lastSemi := strings.LastIndex(inner, ";")
	if lastSemi < 0 {
		return "", 0, fmt.Errorf("invalid array type format: %s", dt)
	}
	elementType = DataType(strings.TrimSpace(inner[:lastSemi]))
	if !elementType.IsValid() && !elementType.IsStructType() && !elementType.IsArrayType() {
		return "", 0, fmt.Errorf("invalid element type: %s", elementType)
	}
	size, err = strconv.Atoi(strings.TrimSpace(inner[lastSemi+1:]))
	if err != nil || size <= 0 {
		return "", 0, fmt.Errorf("invalid array size: %s", inner[lastSemi+1:])
	}
	return elementType, size, nil
}

// NewArrayType は配列型の DataType 文字列を IEC 61131-3 形式で生成する
// 例: NewArrayType("INT", 10)              → "ARRAY[0..9] OF INT"
//
//	NewArrayType("ARRAY[0..4] OF INT", 3) → "ARRAY[0..2, 0..4] OF INT" (フラット化)
func NewArrayType(elementType DataType, size int) DataType {
	s := string(elementType)
	// 要素型が IEC 61131-3 形式の配列の場合はフラット化（多次元表記に統合）
	if ofIdx := strings.Index(s, "] OF "); strings.HasPrefix(s, "ARRAY[") && ofIdx >= 0 {
		innerDims := s[6:ofIdx]  // 内側の次元部分
		innerElem := s[ofIdx+5:] // 内側の要素型
		return DataType(fmt.Sprintf("ARRAY[0..%d, %s] OF %s", size-1, innerDims, innerElem))
	}
	return DataType(fmt.Sprintf("ARRAY[0..%d] OF %s", size-1, elementType))
}

// TypeResolver は複合型のワード数を解決するためのインターフェース
type TypeResolver interface {
	ResolveStructWordCount(typeName string) (int, error)
	ResolveStructDef(typeName string) (*StructTypeDef, error)
}

// WordCountWithResolver は構造体型を含むワード数を返す
func (dt DataType) WordCountWithResolver(resolver TypeResolver) int {
	if dt.IsArrayType() {
		elemType, size, err := ParseArrayType(dt)
		if err != nil {
			return 0
		}
		return elemType.WordCountWithResolver(resolver) * size
	}
	if dt.IsStructType() && resolver != nil {
		wc, err := resolver.ResolveStructWordCount(string(dt))
		if err != nil {
			return 0
		}
		return wc
	}
	return dt.WordCount()
}
