package variable

import (
	"fmt"
	"strings"
)

// StructField は構造体のフィールド定義
type StructField struct {
	Name     string   `json:"name"`
	DataType DataType `json:"dataType"`
	Offset   int      `json:"offset"` // 先頭からのワードオフセット（自動計算）
}

// StructTypeDef は構造体型定義
type StructTypeDef struct {
	Name      string        `json:"name"`
	Fields    []StructField `json:"fields"`
	WordCount int           `json:"wordCount"`
}

// NewStructTypeDef は新しい構造体型定義を作成する
func NewStructTypeDef(name string, fields []StructField, resolver TypeResolver) (*StructTypeDef, error) {
	if name == "" {
		return nil, fmt.Errorf("struct type name is required")
	}
	// 予約語チェック
	if DataType(name).IsValid() || strings.HasPrefix(name, "ARRAY[") {
		return nil, fmt.Errorf("reserved type name: %s", name)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("struct type must have at least one field")
	}

	// フィールド名の重複チェック
	seen := make(map[string]bool)
	for _, f := range fields {
		if f.Name == "" {
			return nil, fmt.Errorf("field name is required")
		}
		if seen[f.Name] {
			return nil, fmt.Errorf("duplicate field name: %s", f.Name)
		}
		seen[f.Name] = true

		// フィールド型のバリデーション
		if f.DataType.IsStructType() {
			// 構造体フィールド: resolverで存在確認
			if resolver != nil {
				if _, err := resolver.ResolveStructDef(string(f.DataType)); err != nil {
					return nil, fmt.Errorf("unknown struct type for field %s: %s", f.Name, f.DataType)
				}
			}
		} else if f.DataType.IsArrayType() {
			elemType, _, err := ParseArrayType(f.DataType)
			if err != nil {
				return nil, fmt.Errorf("invalid array field type: %s for field %s: %w", f.DataType, f.Name, err)
			}
			// 配列要素が構造体の場合、存在確認
			if elemType.IsStructType() && resolver != nil {
				if _, err := resolver.ResolveStructDef(string(elemType)); err != nil {
					return nil, fmt.Errorf("unknown struct element type for field %s: %s", f.Name, elemType)
				}
			}
		} else if !f.DataType.IsValid() {
			return nil, fmt.Errorf("invalid field type: %s for field %s", f.DataType, f.Name)
		}
	}

	// 循環参照チェック
	if resolver != nil {
		if err := detectCyclicDependency(name, fields, resolver, nil); err != nil {
			return nil, err
		}
	}

	// オフセットとWordCountを計算
	offset := 0
	resolvedFields := make([]StructField, len(fields))
	for i, f := range fields {
		resolvedFields[i] = StructField{
			Name:     f.Name,
			DataType: f.DataType,
			Offset:   offset,
		}
		offset += f.DataType.WordCountWithResolver(resolver)
	}

	return &StructTypeDef{
		Name:      name,
		Fields:    resolvedFields,
		WordCount: offset,
	}, nil
}

// detectCyclicDependency は循環参照を検出する
func detectCyclicDependency(typeName string, fields []StructField, resolver TypeResolver, visited map[string]bool) error {
	if visited == nil {
		visited = make(map[string]bool)
	}
	if visited[typeName] {
		return fmt.Errorf("circular dependency detected: %s", typeName)
	}
	visited[typeName] = true

	for _, f := range fields {
		refType := f.DataType
		// 配列の場合は要素型を取得
		if refType.IsArrayType() {
			elemType, _, err := ParseArrayType(refType)
			if err != nil {
				continue
			}
			refType = elemType
		}
		if refType.IsStructType() {
			nestedDef, err := resolver.ResolveStructDef(string(refType))
			if err != nil {
				continue // 存在しない型は別のバリデーションで弾く
			}
			// 再帰チェック（visitedをコピーして渡す）
			visitedCopy := make(map[string]bool)
			for k, v := range visited {
				visitedCopy[k] = v
			}
			if err := detectCyclicDependency(string(refType), nestedDef.Fields, resolver, visitedCopy); err != nil {
				return err
			}
		}
	}
	return nil
}

// DefaultValue は構造体型のデフォルト値を返す
func (s *StructTypeDef) DefaultValue() map[string]interface{} {
	return s.DefaultValueWithResolver(nil)
}

// DefaultValueWithResolver はresolver付きで構造体型のデフォルト値を返す
func (s *StructTypeDef) DefaultValueWithResolver(resolver TypeResolver) map[string]interface{} {
	result := make(map[string]interface{})
	for _, f := range s.Fields {
		if f.DataType.IsArrayType() {
			elemType, size, err := ParseArrayType(f.DataType)
			if err != nil {
				result[f.Name] = nil
				continue
			}
			arr := make([]interface{}, size)
			for i := range arr {
				if elemType.IsStructType() && resolver != nil {
					nestedDef, err := resolver.ResolveStructDef(string(elemType))
					if err != nil {
						arr[i] = nil
					} else {
						arr[i] = nestedDef.DefaultValueWithResolver(resolver)
					}
				} else {
					arr[i] = elemType.DefaultValue()
				}
			}
			result[f.Name] = arr
		} else if f.DataType.IsStructType() && resolver != nil {
			nestedDef, err := resolver.ResolveStructDef(string(f.DataType))
			if err != nil {
				result[f.Name] = nil
			} else {
				result[f.Name] = nestedDef.DefaultValueWithResolver(resolver)
			}
		} else {
			result[f.Name] = f.DataType.DefaultValue()
		}
	}
	return result
}
