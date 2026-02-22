package variable

import (
	"fmt"
	"strconv"
	"strings"
)

// DataType はIEC 61131-3データ型
type DataType string

const (
	TypeBOOL   DataType = "BOOL"   // ブール値
	TypeSINT   DataType = "SINT"   // 符号付き8ビット整数
	TypeINT    DataType = "INT"    // 符号付き16ビット整数
	TypeDINT   DataType = "DINT"   // 符号付き32ビット整数
	TypeUSINT  DataType = "USINT"  // 符号なし8ビット整数
	TypeUINT   DataType = "UINT"   // 符号なし16ビット整数
	TypeUDINT  DataType = "UDINT"  // 符号なし32ビット整数
	TypeREAL   DataType = "REAL"   // 32ビット浮動小数点
	TypeLREAL  DataType = "LREAL"  // 64ビット浮動小数点
	TypeSTRING DataType = "STRING" // 文字列
)

// AllDataTypes はすべてのデータ型を返す
func AllDataTypes() []DataType {
	return []DataType{
		TypeBOOL, TypeSINT, TypeINT, TypeDINT,
		TypeUSINT, TypeUINT, TypeUDINT,
		TypeREAL, TypeLREAL, TypeSTRING,
	}
}

// WordCount はデータ型が占有するワード数を返す
func (dt DataType) WordCount() int {
	switch dt {
	case TypeBOOL, TypeSINT, TypeUSINT, TypeINT, TypeUINT:
		return 1
	case TypeDINT, TypeUDINT, TypeREAL:
		return 2
	case TypeLREAL:
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
	case TypeREAL:
		return float32(0)
	case TypeLREAL:
		return float64(0)
	case TypeSTRING:
		return ""
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
// 例: "ARRAY[INT;10]"
func (dt DataType) IsArrayType() bool {
	return strings.HasPrefix(string(dt), "ARRAY[") && strings.HasSuffix(string(dt), "]")
}

// IsStructType は構造体型かどうかを判定する（スカラーでも配列でもない場合）
func (dt DataType) IsStructType() bool {
	return !dt.IsValid() && !dt.IsArrayType() && string(dt) != ""
}

// ParseArrayType は配列型から要素型とサイズを取得する
// 例: "ARRAY[INT;10]" → (TypeINT, 10, nil)
func ParseArrayType(dt DataType) (elementType DataType, size int, err error) {
	s := string(dt)
	if !strings.HasPrefix(s, "ARRAY[") || !strings.HasSuffix(s, "]") {
		return "", 0, fmt.Errorf("not an array type: %s", dt)
	}
	inner := s[6 : len(s)-1] // "INT;10"
	parts := strings.Split(inner, ";")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid array type format: %s", dt)
	}
	elementType = DataType(strings.TrimSpace(parts[0]))
	if !elementType.IsValid() && !elementType.IsStructType() {
		return "", 0, fmt.Errorf("invalid element type: %s", elementType)
	}
	size, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || size <= 0 {
		return "", 0, fmt.Errorf("invalid array size: %s", parts[1])
	}
	return elementType, size, nil
}

// NewArrayType は配列型のDataType文字列を生成する
func NewArrayType(elementType DataType, size int) DataType {
	return DataType(fmt.Sprintf("ARRAY[%s;%d]", elementType, size))
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
