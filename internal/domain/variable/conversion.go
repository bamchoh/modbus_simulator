package variable

import (
	"encoding/binary"
	"fmt"
	"math"
)

// ConvertValue は値を指定されたデータ型に変換する
func ConvertValue(value interface{}, dataType DataType) (interface{}, error) {
	switch dataType {
	case TypeBOOL:
		switch v := value.(type) {
		case bool:
			return v, nil
		case float64:
			return v != 0, nil
		case int:
			return v != 0, nil
		case int64:
			return v != 0, nil
		}
	case TypeSINT:
		switch v := value.(type) {
		case int8:
			return v, nil
		case float64:
			return int8(v), nil
		case int:
			return int8(v), nil
		case int64:
			return int8(v), nil
		}
	case TypeINT:
		switch v := value.(type) {
		case int16:
			return v, nil
		case float64:
			return int16(v), nil
		case int:
			return int16(v), nil
		case int64:
			return int16(v), nil
		}
	case TypeDINT:
		switch v := value.(type) {
		case int32:
			return v, nil
		case float64:
			return int32(v), nil
		case int:
			return int32(v), nil
		case int64:
			return int32(v), nil
		}
	case TypeUSINT:
		switch v := value.(type) {
		case uint8:
			return v, nil
		case float64:
			return uint8(v), nil
		case int:
			return uint8(v), nil
		case int64:
			return uint8(v), nil
		}
	case TypeUINT:
		switch v := value.(type) {
		case uint16:
			return v, nil
		case float64:
			return uint16(v), nil
		case int:
			return uint16(v), nil
		case int64:
			return uint16(v), nil
		}
	case TypeUDINT:
		switch v := value.(type) {
		case uint32:
			return v, nil
		case float64:
			return uint32(v), nil
		case int:
			return uint32(v), nil
		case int64:
			return uint32(v), nil
		}
	case TypeREAL:
		switch v := value.(type) {
		case float32:
			return v, nil
		case float64:
			return float32(v), nil
		case int:
			return float32(v), nil
		case int64:
			return float32(v), nil
		}
	case TypeLREAL:
		switch v := value.(type) {
		case float64:
			return v, nil
		case float32:
			return float64(v), nil
		case int:
			return float64(v), nil
		case int64:
			return float64(v), nil
		}
	case TypeSTRING:
		switch v := value.(type) {
		case string:
			return v, nil
		}
	}

	// STRING[n] 型
	if dataType.IsStringType() {
		switch v := value.(type) {
		case string:
			maxLen, _ := ParseStringType(dataType)
			if maxLen > 0 && len(v) > maxLen {
				v = v[:maxLen]
			}
			return v, nil
		}
		return nil, fmt.Errorf("cannot convert %T to %s", value, dataType)
	}

	// 配列型
	if dataType.IsArrayType() {
		return convertArrayValue(value, dataType)
	}

	// 構造体型（map[string]interface{}ならそのまま受け入れ）
	if dataType.IsStructType() {
		if m, ok := value.(map[string]interface{}); ok {
			return m, nil
		}
		return nil, fmt.Errorf("cannot convert %T to struct %s", value, dataType)
	}

	return nil, fmt.Errorf("cannot convert %T to %s", value, dataType)
}

// ConvertValueWithResolver はresolver付きで値を再帰的に変換する
// 構造体フィールドや配列要素の型変換を正しく行う
func ConvertValueWithResolver(value interface{}, dataType DataType, resolver TypeResolver) (interface{}, error) {
	// スカラー型はConvertValueと同じ
	if dataType.IsValid() {
		return ConvertValue(value, dataType)
	}

	// 配列型
	if dataType.IsArrayType() {
		elemType, size, err := ParseArrayType(dataType)
		if err != nil {
			return nil, err
		}
		arr, ok := value.([]interface{})
		if !ok {
			return nil, fmt.Errorf("array value must be []interface{}, got %T", value)
		}
		if len(arr) != size {
			return nil, fmt.Errorf("array size mismatch: expected %d, got %d", size, len(arr))
		}
		result := make([]interface{}, size)
		for i, v := range arr {
			converted, err := ConvertValueWithResolver(v, elemType, resolver)
			if err != nil {
				return nil, fmt.Errorf("element[%d]: %w", i, err)
			}
			result[i] = converted
		}
		return result, nil
	}

	// 構造体型
	if dataType.IsStructType() && resolver != nil {
		m, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("cannot convert %T to struct %s", value, dataType)
		}
		structDef, err := resolver.ResolveStructDef(string(dataType))
		if err != nil {
			return nil, fmt.Errorf("unknown struct type: %s", dataType)
		}
		result := make(map[string]interface{})
		for _, field := range structDef.Fields {
			fieldVal, exists := m[field.Name]
			if !exists {
				result[field.Name] = field.DataType.DefaultValue()
				continue
			}
			converted, err := ConvertValueWithResolver(fieldVal, field.DataType, resolver)
			if err != nil {
				result[field.Name] = field.DataType.DefaultValue()
				continue
			}
			result[field.Name] = converted
		}
		return result, nil
	}

	return ConvertValue(value, dataType)
}

// convertArrayValue は配列型の値を変換する
func convertArrayValue(value interface{}, dt DataType) (interface{}, error) {
	elemType, size, err := ParseArrayType(dt)
	if err != nil {
		return nil, err
	}

	arr, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("array value must be []interface{}, got %T", value)
	}
	if len(arr) != size {
		return nil, fmt.Errorf("array size mismatch: expected %d, got %d", size, len(arr))
	}

	result := make([]interface{}, size)
	for i, v := range arr {
		converted, err := ConvertValue(v, elemType)
		if err != nil {
			return nil, fmt.Errorf("element[%d]: %w", i, err)
		}
		result[i] = converted
	}
	return result, nil
}

// ValueToWords は変数の値をワード列に変換する（ビッグエンディアン）
func ValueToWords(value interface{}, dataType DataType, endianness string) []uint16 {
	switch dataType {
	case TypeBOOL:
		if val, ok := value.(bool); ok && val {
			return []uint16{1}
		}
		return []uint16{0}
	case TypeSINT:
		if val, ok := value.(int8); ok {
			return []uint16{uint16(val)}
		}
	case TypeINT:
		if val, ok := value.(int16); ok {
			return []uint16{uint16(val)}
		}
	case TypeUSINT:
		if val, ok := value.(uint8); ok {
			return []uint16{uint16(val)}
		}
	case TypeUINT:
		if val, ok := value.(uint16); ok {
			return []uint16{val}
		}
	case TypeDINT:
		if val, ok := value.(int32); ok {
			return uint32ToWords(uint32(val), endianness)
		}
	case TypeUDINT:
		if val, ok := value.(uint32); ok {
			return uint32ToWords(val, endianness)
		}
	case TypeREAL:
		if val, ok := value.(float32); ok {
			bits := math.Float32bits(val)
			return uint32ToWords(bits, endianness)
		}
	case TypeLREAL:
		if val, ok := value.(float64); ok {
			bits := math.Float64bits(val)
			return uint64ToWords(bits, endianness)
		}
	case TypeSTRING:
		if val, ok := value.(string); ok {
			return stringToWords(val)
		}
	}

	// STRING[n] 型
	if dataType.IsStringType() {
		if val, ok := value.(string); ok {
			maxLen, err := ParseStringType(dataType)
			if err == nil {
				return stringToWordsFixed(val, maxLen)
			}
		}
	}

	return []uint16{0}
}

// WordsToValue はワード列を変数の値に変換する
func WordsToValue(words []uint16, dataType DataType, endianness string) (interface{}, error) {
	if len(words) == 0 {
		return dataType.DefaultValue(), nil
	}

	switch dataType {
	case TypeBOOL:
		return words[0] != 0, nil
	case TypeSINT:
		return int8(words[0]), nil
	case TypeINT:
		return int16(words[0]), nil
	case TypeUSINT:
		return uint8(words[0]), nil
	case TypeUINT:
		return words[0], nil
	case TypeDINT:
		if len(words) < 2 {
			return int32(int16(words[0])), nil
		}
		return int32(wordsToUint32(words[:2], endianness)), nil
	case TypeUDINT:
		if len(words) < 2 {
			return uint32(words[0]), nil
		}
		return wordsToUint32(words[:2], endianness), nil
	case TypeREAL:
		if len(words) < 2 {
			return float32(0), nil
		}
		bits := wordsToUint32(words[:2], endianness)
		return math.Float32frombits(bits), nil
	case TypeLREAL:
		if len(words) < 4 {
			return float64(0), nil
		}
		bits := wordsToUint64(words[:4], endianness)
		return math.Float64frombits(bits), nil
	case TypeSTRING:
		return wordsToString(words), nil
	}

	// STRING[n] 型
	if dataType.IsStringType() {
		maxLen, err := ParseStringType(dataType)
		if err == nil {
			return wordsToStringFixed(words, maxLen), nil
		}
	}

	return nil, fmt.Errorf("unsupported data type: %s", dataType)
}

// ValueToBool は変数の値をブール値に変換する
func ValueToBool(value interface{}, dataType DataType) bool {
	switch dataType {
	case TypeBOOL:
		if val, ok := value.(bool); ok {
			return val
		}
	default:
		words := ValueToWords(value, dataType, "big")
		if len(words) > 0 {
			return words[0] != 0
		}
	}
	return false
}

// uint32ToWords は32ビット値を2ワードに分割する
func uint32ToWords(val uint32, endianness string) []uint16 {
	hi := uint16(val >> 16)
	lo := uint16(val & 0xFFFF)
	if endianness == "little" {
		return []uint16{lo, hi}
	}
	// ビッグエンディアン: 上位ワードが先
	return []uint16{hi, lo}
}

// uint64ToWords は64ビット値を4ワードに分割する
func uint64ToWords(val uint64, endianness string) []uint16 {
	w0 := uint16(val >> 48)
	w1 := uint16(val >> 32)
	w2 := uint16(val >> 16)
	w3 := uint16(val)
	if endianness == "little" {
		return []uint16{w3, w2, w1, w0}
	}
	return []uint16{w0, w1, w2, w3}
}

// wordsToUint32 は2ワードを32ビット値に結合する
func wordsToUint32(words []uint16, endianness string) uint32 {
	if endianness == "little" {
		return uint32(words[1])<<16 | uint32(words[0])
	}
	return uint32(words[0])<<16 | uint32(words[1])
}

// wordsToUint64 は4ワードを64ビット値に結合する
func wordsToUint64(words []uint16, endianness string) uint64 {
	if endianness == "little" {
		return uint64(words[3])<<48 | uint64(words[2])<<32 | uint64(words[1])<<16 | uint64(words[0])
	}
	return uint64(words[0])<<48 | uint64(words[1])<<32 | uint64(words[2])<<16 | uint64(words[3])
}

// stringToWords は文字列をワード列に変換する（長さプレフィックス付き）
func stringToWords(s string) []uint16 {
	strBytes := []byte(s)
	// 長さ(4バイト) + データ
	buf := make([]byte, 4+len(strBytes))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(strBytes)))
	copy(buf[4:], strBytes)

	// 2バイト単位でワードに変換（パディング）
	wordCount := (len(buf) + 1) / 2
	words := make([]uint16, wordCount)
	for i := 0; i < wordCount; i++ {
		hi := buf[i*2]
		var lo byte
		if i*2+1 < len(buf) {
			lo = buf[i*2+1]
		}
		words[i] = uint16(hi)<<8 | uint16(lo)
	}
	return words
}

// stringToWordsFixed は固定長文字列をワード列に変換する（ヌルパディング）
func stringToWordsFixed(s string, maxBytes int) []uint16 {
	buf := make([]byte, maxBytes)
	copy(buf, []byte(s))
	// 残りはゼロ（ヌルパディング）
	wordCount := (maxBytes + 1) / 2
	words := make([]uint16, wordCount)
	for i := 0; i < wordCount; i++ {
		hi := buf[i*2]
		var lo byte
		if i*2+1 < maxBytes {
			lo = buf[i*2+1]
		}
		words[i] = uint16(hi)<<8 | uint16(lo)
	}
	return words
}

// wordsToStringFixed はワード列を固定長文字列に変換する（ヌル文字で終端）
func wordsToStringFixed(words []uint16, maxBytes int) string {
	buf := make([]byte, len(words)*2)
	for i, w := range words {
		buf[i*2] = byte(w >> 8)
		buf[i*2+1] = byte(w)
	}
	if len(buf) > maxBytes {
		buf = buf[:maxBytes]
	}
	// ヌル文字で終端
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// ArrayValueToWords は配列値をワード列に変換する
func ArrayValueToWords(value interface{}, elemType DataType, size int, endianness string, resolver TypeResolver) []uint16 {
	arr, ok := value.([]interface{})
	if !ok {
		return make([]uint16, elemType.WordCountWithResolver(resolver)*size)
	}
	var words []uint16
	for _, elem := range arr {
		var elemWords []uint16
		if elemType.IsStructType() && resolver != nil {
			structDef, err := resolver.ResolveStructDef(string(elemType))
			if err == nil {
				elemWords = StructValueToWords(elem, structDef, endianness, resolver)
			} else {
				elemWords = make([]uint16, elemType.WordCountWithResolver(resolver))
			}
		} else {
			elemWords = ValueToWords(elem, elemType, endianness)
		}
		words = append(words, elemWords...)
	}
	return words
}

// WordsToArrayValue はワード列を配列値に変換する
func WordsToArrayValue(words []uint16, elemType DataType, size int, endianness string, resolver TypeResolver) ([]interface{}, error) {
	elemWC := elemType.WordCountWithResolver(resolver)
	result := make([]interface{}, size)
	for i := 0; i < size; i++ {
		offset := i * elemWC
		if offset+elemWC > len(words) {
			if elemType.IsStructType() && resolver != nil {
				if structDef, err := resolver.ResolveStructDef(string(elemType)); err == nil {
					result[i] = structDef.DefaultValueWithResolver(resolver)
				}
			} else {
				result[i] = elemType.DefaultValue()
			}
			continue
		}
		if elemType.IsStructType() && resolver != nil {
			structDef, err := resolver.ResolveStructDef(string(elemType))
			if err != nil {
				result[i] = nil
				continue
			}
			val, err := WordsToStructValue(words[offset:offset+elemWC], structDef, endianness, resolver)
			if err != nil {
				result[i] = structDef.DefaultValueWithResolver(resolver)
				continue
			}
			result[i] = val
		} else {
			val, err := WordsToValue(words[offset:offset+elemWC], elemType, endianness)
			if err != nil {
				result[i] = elemType.DefaultValue()
				continue
			}
			result[i] = val
		}
	}
	return result, nil
}

// StructValueToWords は構造体値をワード列に変換する
func StructValueToWords(value interface{}, structDef *StructTypeDef, endianness string, resolver TypeResolver) []uint16 {
	m, ok := value.(map[string]interface{})
	if !ok {
		return make([]uint16, structDef.WordCount)
	}
	words := make([]uint16, structDef.WordCount)
	for _, field := range structDef.Fields {
		fieldVal := m[field.Name]
		var fieldWords []uint16
		if field.DataType.IsArrayType() {
			elemType, size, err := ParseArrayType(field.DataType)
			if err != nil {
				continue
			}
			fieldWords = ArrayValueToWords(fieldVal, elemType, size, endianness, resolver)
		} else if field.DataType.IsStructType() && resolver != nil {
			nestedDef, err := resolver.ResolveStructDef(string(field.DataType))
			if err != nil {
				continue
			}
			fieldWords = StructValueToWords(fieldVal, nestedDef, endianness, resolver)
		} else {
			fieldWords = ValueToWords(fieldVal, field.DataType, endianness)
		}
		copy(words[field.Offset:], fieldWords)
	}
	return words
}

// WordsToStructValue はワード列を構造体値に変換する
func WordsToStructValue(words []uint16, structDef *StructTypeDef, endianness string, resolver TypeResolver) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, field := range structDef.Fields {
		var wc int
		if field.DataType.IsArrayType() {
			elemType, size, err := ParseArrayType(field.DataType)
			if err != nil {
				continue
			}
			wc = elemType.WordCountWithResolver(resolver) * size
			if field.Offset+wc > len(words) {
				result[field.Name] = nil
				continue
			}
			val, err := WordsToArrayValue(words[field.Offset:field.Offset+wc], elemType, size, endianness, resolver)
			if err != nil {
				result[field.Name] = nil
				continue
			}
			result[field.Name] = val
		} else if field.DataType.IsStructType() && resolver != nil {
			nestedDef, err := resolver.ResolveStructDef(string(field.DataType))
			if err != nil {
				continue
			}
			wc = nestedDef.WordCount
			if field.Offset+wc > len(words) {
				result[field.Name] = nestedDef.DefaultValueWithResolver(resolver)
				continue
			}
			val, err := WordsToStructValue(words[field.Offset:field.Offset+wc], nestedDef, endianness, resolver)
			if err != nil {
				result[field.Name] = nestedDef.DefaultValueWithResolver(resolver)
				continue
			}
			result[field.Name] = val
		} else {
			wc = field.DataType.WordCount()
			if field.Offset+wc > len(words) {
				result[field.Name] = field.DataType.DefaultValue()
				continue
			}
			val, err := WordsToValue(words[field.Offset:field.Offset+wc], field.DataType, endianness)
			if err != nil {
				result[field.Name] = field.DataType.DefaultValue()
				continue
			}
			result[field.Name] = val
		}
	}
	return result, nil
}

// wordsToString はワード列を文字列に変換する
func wordsToString(words []uint16) string {
	if len(words) < 2 {
		return ""
	}
	// ワードをバイト列に変換
	buf := make([]byte, len(words)*2)
	for i, w := range words {
		buf[i*2] = byte(w >> 8)
		buf[i*2+1] = byte(w)
	}
	// 最初の4バイトは長さ
	if len(buf) < 4 {
		return ""
	}
	length := binary.BigEndian.Uint32(buf[:4])
	if int(length) > len(buf)-4 {
		length = uint32(len(buf) - 4)
	}
	return string(buf[4 : 4+length])
}
