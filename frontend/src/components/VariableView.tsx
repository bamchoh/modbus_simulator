import { useState, useEffect, useCallback } from 'react';
import {
  GetVariables,
  GetDataTypes,
  CreateVariable,
  UpdateVariableValue,
  DeleteVariable,
  UpdateVariableMappings,
  GetAvailableProtocols,
  GetMemoryAreas,
  GetStructTypes,
  RegisterStructType,
  DeleteStructType,
} from '../../wailsjs/go/main/App';
import { application } from '../../wailsjs/go/models';

interface VariableViewProps {
  autoRefresh?: boolean;
}

// 構造体型配列の要素エディタ（ローカルで展開/折りたたみ状態を管理）
const StructArrayElementEditor = ({
  elemType,
  elem,
  onChange,
  renderEditor
}: {
  elemType: string;
  elem: any;
  onChange: (v: any) => void;
  renderEditor: (dataType: string, value: any, onChange: (v: any) => void) => JSX.Element;
}) => {
  const [isExpanded, setIsExpanded] = useState(false);
  return (
    <div style={{ border: '1px solid #444', borderRadius: '3px', padding: '2px 4px' }}>
      <div
        style={{ display: 'flex', alignItems: 'center', cursor: 'pointer', gap: '4px' }}
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <span style={{ fontSize: '0.8em' }}>{isExpanded ? '\u25BC' : '\u25B6'}</span>
        <span style={{ fontSize: '0.8em', color: '#aaa' }}>{elemType}</span>
      </div>
      {isExpanded && (
        <div style={{ marginTop: '4px' }}>
          {renderEditor(elemType, elem, onChange)}
        </div>
      )}
    </div>
  );
};

export function VariableView({ autoRefresh = true }: VariableViewProps) {
  const [variables, setVariables] = useState<application.VariableDTO[]>([]);
  const [dataTypes, setDataTypes] = useState<application.DataTypeInfoDTO[]>([]);
  const [structTypes, setStructTypes] = useState<application.StructTypeDTO[]>([]);
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isMappingDialogOpen, setIsMappingDialogOpen] = useState(false);
  const [isStructTypeDialogOpen, setIsStructTypeDialogOpen] = useState(false);
  const [editingVariable, setEditingVariable] = useState<application.VariableDTO | null>(null);

  // 新規変数フォーム
  const [newName, setNewName] = useState('');
  const [newDataType, setNewDataType] = useState('INT');
  const [newValue, setNewValue] = useState('');
  // 配列型追加用
  const [newArrayElemType, setNewArrayElemType] = useState('INT');
  const [newArraySize, setNewArraySize] = useState(10);
  // STRING型のバイト長
  const [newStringLength, setNewStringLength] = useState(20);
  // 型カテゴリ: 'scalar' | 'array' | 'struct'
  const [newTypeCategory, setNewTypeCategory] = useState<'scalar' | 'array' | 'struct'>('scalar');

  // 構造体型定義フォーム
  const [structTypeName, setStructTypeName] = useState('');
  const [editingStructTypeName, setEditingStructTypeName] = useState<string | null>(null);
  const [structTypeFields, setStructTypeFields] = useState<{
    name: string;
    category: 'scalar' | 'struct' | 'array';
    dataType: string;
    stringLength: number;
    arrayElemType: string;
    arrayElemCategory: 'scalar' | 'struct';
    arraySize: number;
  }[]>([
    { name: '', category: 'scalar', dataType: 'INT', stringLength: 20, arrayElemType: 'INT', arrayElemCategory: 'scalar', arraySize: 10 },
  ]);

  // 展開/縮小状態（keyはFlatRowのヘッダーキー）
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());

  // 編集フォーム
  const [editData, setEditData] = useState<any>(null);
  const [editValue, setEditValue] = useState('');

  // マッピング編集
  const [mappingVariable, setMappingVariable] = useState<application.VariableDTO | null>(null);
  const [editMappings, setEditMappings] = useState<application.ProtocolMappingDTO[]>([]);
  const [protocols, setProtocols] = useState<application.ProtocolInfoDTO[]>([]);
  const [memoryAreas, setMemoryAreas] = useState<application.MemoryAreaDTO[]>([]);

  // 変数一覧を取得
  const loadVariables = useCallback(async () => {
    try {
      const vars = await GetVariables();
      if (vars) {
        setVariables(vars);
      }
    } catch (e) {
      console.error('Failed to load variables:', e);
    }
  }, []);

  // 構造体型一覧を取得
  const loadStructTypes = useCallback(async () => {
    try {
      const types = await GetStructTypes();
      setStructTypes(types || []);
    } catch (e) {
      console.error('Failed to load struct types:', e);
    }
  }, []);

  // データ型一覧を取得
  useEffect(() => {
    const loadDataTypes = async () => {
      try {
        const types = await GetDataTypes();
        if (types && types.types) {
          setDataTypes(types.types);
        }
      } catch (e) {
        console.error('Failed to load data types:', e);
      }
    };
    loadDataTypes();
    loadStructTypes();

    const loadProtocols = async () => {
      try {
        const protos = await GetAvailableProtocols();
        if (protos) {
          setProtocols(protos);
        }
      } catch (e) {
        console.error('Failed to load protocols:', e);
      }
    };
    loadProtocols();

    const loadAreas = async () => {
      try {
        const areas = await GetMemoryAreas();
        if (areas) {
          setMemoryAreas(areas);
        }
      } catch (e) {
        console.error('Failed to load memory areas:', e);
      }
    };
    loadAreas();
  }, []);

  // 変数一覧の初回読み込み
  useEffect(() => {
    loadVariables();
  }, [loadVariables]);

  // 自動更新
  useEffect(() => {
    if (autoRefresh) {
      const interval = setInterval(loadVariables, 500);
      return () => clearInterval(interval);
    }
  }, [autoRefresh, loadVariables]);

  // スカラー値のフォーマット
  const formatScalarValue = (value: any, dataType: string): string => {
    if (value === null || value === undefined) return '-';
    switch (dataType) {
      case 'BOOL':
        return value ? 'TRUE' : 'FALSE';
      case 'STRING':
        return `"${value}"`;
      case 'REAL':
      case 'LREAL':
        return typeof value === 'number' ? value.toFixed(2) : String(value);
      default:
        if (dataType.startsWith('STRING[')) return `"${value}"`;
        return String(value);
    }
  };

  // 構造体型かどうか判定
  const isStructType = (dataType: string): boolean => {
    const scalarTypes = ['BOOL', 'SINT', 'INT', 'DINT', 'USINT', 'UINT', 'UDINT', 'REAL', 'LREAL', 'STRING', 'TIME', 'DATE', 'TIME_OF_DAY', 'DATE_AND_TIME'];
    return !scalarTypes.includes(dataType) && !dataType.startsWith('ARRAY[') && !dataType.startsWith('STRING[');
  };

  // データ型のワード数を取得
  const getWordCount = (dataType: string): number => {
    // STRING[n] 型
    const strMatch = dataType.match(/^STRING\[(\d+)\]$/);
    if (strMatch) {
      const maxBytes = parseInt(strMatch[1]);
      return Math.ceil(maxBytes / 2);
    }
    // スカラー型
    const dt = dataTypes.find(t => t.id === dataType);
    if (dt) return dt.wordCount;
    // 構造体型
    const st = structTypes.find(s => s.name === dataType);
    if (st) return st.wordCount;
    // 配列型
    const match = dataType.match(/^ARRAY\[(.+);(\d+)\]$/);
    if (match) {
      const elemWc = getWordCount(match[1]);
      return elemWc * parseInt(match[2]);
    }
    return 1;
  };

  // フラット化された行の型定義
  interface FlatRow {
    key: string;
    displayName: string;
    dataType: string;
    value: any;
    depth: number;
    // 親変数への参照（編集・削除・マッピング用）
    variable: application.VariableDTO;
    // 値のパスを表すキー配列（例: ["fieldName"] や [0, "fieldName"]）
    valuePath: (string | number)[];
    // この行がヘッダー行（構造体/配列の親）かどうか
    isHeader: boolean;
    // 変数のベースアドレスからのワードオフセット
    wordOffset: number;
  }

  // 変数をフラットな行に展開
  const flattenVariable = (v: application.VariableDTO): FlatRow[] => {
    const rows: FlatRow[] = [];

    const expand = (
      displayName: string,
      dataType: string,
      value: any,
      depth: number,
      valuePath: (string | number)[],
      wordOffset: number,
    ) => {
      // 配列型
      if (dataType.startsWith('ARRAY[')) {
        const match = dataType.match(/^ARRAY\[(.+);(\d+)\]$/);
        const elemType = match ? match[1] : '';
        const elemWordCount = getWordCount(elemType);
        // 親ヘッダー行
        rows.push({
          key: `${v.id}:${valuePath.join('.')}:header`,
          displayName,
          dataType,
          value,
          depth,
          variable: v,
          valuePath,
          isHeader: true,
          wordOffset,
        });
        // 各要素を行として展開
        if (Array.isArray(value)) {
          value.forEach((elem: any, i: number) => {
            const elemPath = [...valuePath, i];
            const elemName = `${displayName}[${i}]`;
            const elemOffset = wordOffset + i * elemWordCount;
            if (isStructType(elemType)) {
              expand(elemName, elemType, elem, depth + 1, elemPath, elemOffset);
            } else {
              rows.push({
                key: `${v.id}:${elemPath.join('.')}`,
                displayName: elemName,
                dataType: elemType,
                value: elem,
                depth: depth + 1,
                variable: v,
                valuePath: elemPath,
                isHeader: false,
                wordOffset: elemOffset,
              });
            }
          });
        }
        return;
      }

      // 構造体型
      if (isStructType(dataType) && typeof value === 'object' && value !== null && !Array.isArray(value)) {
        const st = structTypes.find(s => s.name === dataType);
        // 親ヘッダー行
        rows.push({
          key: `${v.id}:${valuePath.join('.')}:header`,
          displayName,
          dataType,
          value,
          depth,
          variable: v,
          valuePath,
          isHeader: true,
          wordOffset,
        });
        if (st) {
          st.fields.forEach((field) => {
            const fieldPath = [...valuePath, field.name];
            const fieldName = `${displayName}.${field.name}`;
            const fieldOffset = wordOffset + field.offset;
            if (isStructType(field.dataType) || field.dataType.startsWith('ARRAY[')) {
              expand(fieldName, field.dataType, value[field.name], depth + 1, fieldPath, fieldOffset);
            } else {
              rows.push({
                key: `${v.id}:${fieldPath.join('.')}`,
                displayName: fieldName,
                dataType: field.dataType,
                value: value[field.name],
                depth: depth + 1,
                variable: v,
                valuePath: fieldPath,
                isHeader: false,
                wordOffset: fieldOffset,
              });
            }
          });
        }
        return;
      }

      // スカラー型
      rows.push({
        key: `${v.id}:${valuePath.join('.')}`,
        displayName,
        dataType,
        value,
        depth,
        variable: v,
        valuePath,
        isHeader: false,
        wordOffset,
      });
    };

    expand(v.name, v.dataType, v.value, 0, [], 0);
    return rows;
  };

  // すべての変数をフラット化
  const allFlatRows = variables.flatMap(flattenVariable);

  // 展開/縮小を考慮して表示する行をフィルタリング
  // 各行の親ヘッダーがすべて展開中であれば表示
  const flatRows = (() => {
    const visible: FlatRow[] = [];
    // 縮小中のヘッダーの depth を追跡（この depth 以下の行を非表示）
    let collapseDepth = -1; // -1 = 非表示なし
    let collapseVarId = '';

    for (const row of allFlatRows) {
      // 別の変数に移ったらリセット
      if (row.variable.id !== collapseVarId) {
        collapseDepth = -1;
      }

      // 縮小中のヘッダーより深い行は非表示
      if (collapseDepth >= 0 && row.variable.id === collapseVarId && row.depth > collapseDepth) {
        continue;
      } else {
        // 抜けたのでリセット
        collapseDepth = -1;
      }

      visible.push(row);

      // ヘッダー行で縮小中なら、以降のこのヘッダーの深さより深い行を非表示にする
      if (row.isHeader && !expandedRows.has(row.key)) {
        collapseDepth = row.depth;
        collapseVarId = row.variable.id;
      }
    }
    return visible;
  })();

  // ヘッダー行の展開/縮小をトグル
  const toggleExpand = (headerKey: string) => {
    setExpandedRows(prev => {
      const next = new Set(prev);
      if (next.has(headerKey)) {
        next.delete(headerKey);
      } else {
        next.add(headerKey);
      }
      return next;
    });
  };

  // デフォルト値を取得
  const getDefaultValue = (dataType: string): string => {
    if (dataType.startsWith('STRING[') || dataType === 'STRING') return '';
    switch (dataType) {
      case 'BOOL': return 'false';
      case 'REAL':
      case 'LREAL': return '0.0';
      default: return '0';
    }
  };

  // 入力値をパース
  const parseValue = (input: string, dataType: string): any => {
    switch (dataType) {
      case 'BOOL':
        return input.toLowerCase() === 'true' || input === '1';
      case 'STRING':
        return input;
      case 'REAL':
      case 'LREAL':
        return parseFloat(input) || 0;
      default:
        if (dataType.startsWith('STRING[')) return input;
        return parseInt(input, 10) || 0;
    }
  };

  // マッピングのフォーマット
  const formatMappings = (mappings: application.ProtocolMappingDTO[] | undefined): string => {
    if (!mappings || mappings.length === 0) return '-';
    return mappings.map(m => `${m.protocolType}:${m.memoryArea}:${m.address}`).join(', ');
  };

  // オフセット付きマッピングのフォーマット（フィールド/要素用）
  const formatMappingsWithOffset = (mappings: application.ProtocolMappingDTO[] | undefined, offset: number): string => {
    if (!mappings || mappings.length === 0) return '-';
    return mappings.map(m => `${m.memoryArea}:${m.address + offset}`).join(', ');
  };

  // 構造体のデフォルト値を再帰的に生成
  const generateStructDefault = (typeName: string): any => {
    const st = structTypes.find(s => s.name === typeName);
    if (!st) return {};
    const obj: Record<string, any> = {};
    for (const f of st.fields) {
      if (f.dataType.startsWith('ARRAY[')) {
        // ARRAY[ElemType;Size] をパース
        const match = f.dataType.match(/^ARRAY\[(.+);(\d+)\]$/);
        if (match) {
          const elemType = match[1];
          const size = parseInt(match[2]);
          const isElemStruct = !['BOOL','SINT','INT','DINT','USINT','UINT','UDINT','REAL','LREAL','STRING'].includes(elemType) && !elemType.startsWith('STRING[');
          obj[f.name] = Array.from({ length: size }, () =>
            isElemStruct ? generateStructDefault(elemType) : parseValue(getDefaultValue(elemType), elemType)
          );
        } else {
          obj[f.name] = [];
        }
      } else if (isStructType(f.dataType)) {
        obj[f.name] = generateStructDefault(f.dataType);
      } else {
        obj[f.name] = parseValue(getDefaultValue(f.dataType), f.dataType);
      }
    }
    return obj;
  };

  // 変数を追加
  const handleAddVariable = async () => {
    if (!newName.trim()) return;
    try {
      let dataType = newDataType;
      let value: any;

      if (newTypeCategory === 'array') {
        const elemType = newArrayElemType === 'STRING' ? `STRING[${newStringLength}]` : newArrayElemType;
        dataType = `ARRAY[${elemType};${newArraySize}]`;
        // デフォルト配列を生成（構造体要素にも対応）
        if (isStructType(newArrayElemType)) {
          value = Array.from({ length: newArraySize }, () => generateStructDefault(newArrayElemType));
        } else {
          const defaultVal = getDefaultValue(elemType);
          value = Array.from({ length: newArraySize }, () => parseValue(defaultVal, elemType));
        }
      } else if (newTypeCategory === 'struct') {
        // 構造体のデフォルト値を再帰的に生成
        value = generateStructDefault(newDataType);
      } else {
        // スカラー型
        // STRING選択時は STRING[n] 形式にする
        if (newDataType === 'STRING') {
          dataType = `STRING[${newStringLength}]`;
        }
        value = parseValue(newValue || getDefaultValue(dataType), dataType);
      }

      await CreateVariable(newName.trim(), dataType, value);
      await loadVariables();
      setIsAddDialogOpen(false);
      setNewName('');
      setNewDataType('INT');
      setNewValue('');
      setNewTypeCategory('scalar');
      setNewArrayElemType('INT');
      setNewArraySize(10);
    } catch (e) {
      console.error('Failed to create variable:', e);
      alert('変数の作成に失敗しました: ' + e);
    }
  };

  // フィールドの実際のデータ型文字列を取得
  const resolveFieldDataType = (field: typeof structTypeFields[0]): string => {
    if (field.category === 'scalar') {
      if (field.dataType === 'STRING') return `STRING[${field.stringLength}]`;
      return field.dataType;
    }
    if (field.category === 'struct') return field.dataType;
    // array
    let elemType = field.arrayElemType;
    if (field.arrayElemCategory === 'scalar' && elemType === 'STRING') {
      elemType = `STRING[${field.stringLength}]`;
    }
    return `ARRAY[${elemType};${field.arraySize}]`;
  };

  // 構造体型を登録または更新
  const handleRegisterStructType = async () => {
    if (!structTypeName.trim()) return;
    const validFields = structTypeFields.filter(f => f.name.trim());
    if (validFields.length === 0) {
      alert('少なくとも1つのフィールドを定義してください。');
      return;
    }
    try {
      // 編集モードの場合、既存の型を削除してから新しい型を登録
      if (editingStructTypeName) {
        await DeleteStructType(editingStructTypeName);
      }
      await RegisterStructType({
        name: structTypeName.trim(),
        fields: validFields.map(f => ({ name: f.name.trim(), dataType: resolveFieldDataType(f), offset: 0 })),
        wordCount: 0,
      } as application.StructTypeDTO);
      await loadStructTypes();
      setStructTypeName('');
      setEditingStructTypeName(null);
      setStructTypeFields([{ name: '', category: 'scalar', dataType: 'INT', stringLength: 20, arrayElemType: 'INT', arrayElemCategory: 'scalar', arraySize: 10 }]);
    } catch (e) {
      console.error('Failed to register struct type:', e);
      alert('構造体型の登録に失敗しました: ' + e);
    }
  };

  // 構造体型を編集
  const handleEditStructType = (st: application.StructTypeDTO) => {
    setEditingStructTypeName(st.name);
    setStructTypeName(st.name);

    // フィールドをフォーム形式に変換
    const formFields = st.fields.map(f => {
      const field: typeof structTypeFields[0] = {
        name: f.name,
        category: 'scalar',
        dataType: f.dataType,
        stringLength: 20,
        arrayElemType: 'INT',
        arrayElemCategory: 'scalar',
        arraySize: 10,
      };

      // データ型を解析してカテゴリを判定
      if (f.dataType.startsWith('ARRAY[')) {
        field.category = 'array';
        const match = f.dataType.match(/^ARRAY\[(.+);(\d+)\]$/);
        if (match) {
          let elemType = match[1];
          field.arraySize = parseInt(match[2]);

          if (elemType.startsWith('STRING[')) {
            field.arrayElemCategory = 'scalar';
            field.arrayElemType = 'STRING';
            const lenMatch = elemType.match(/^STRING\[(\d+)\]$/);
            if (lenMatch) field.stringLength = parseInt(lenMatch[1]);
          } else if (structTypes.some(s => s.name === elemType)) {
            field.arrayElemCategory = 'struct';
            field.arrayElemType = elemType;
          } else {
            field.arrayElemCategory = 'scalar';
            field.arrayElemType = elemType;
          }
        }
      } else if (f.dataType.startsWith('STRING[')) {
        field.category = 'scalar';
        field.dataType = 'STRING';
        const match = f.dataType.match(/^STRING\[(\d+)\]$/);
        if (match) field.stringLength = parseInt(match[1]);
      } else if (structTypes.some(s => s.name === f.dataType)) {
        field.category = 'struct';
        field.dataType = f.dataType;
      } else {
        field.category = 'scalar';
        field.dataType = f.dataType;
      }

      return field;
    });

    setStructTypeFields(formFields);
  };

  // 構造体型を削除
  const handleDeleteStructType = async (name: string) => {
    if (!confirm(`構造体型 "${name}" を削除しますか?`)) return;
    try {
      await DeleteStructType(name);
      await loadStructTypes();
    } catch (e) {
      console.error('Failed to delete struct type:', e);
      alert('構造体型の削除に失敗しました: ' + e);
    }
  };

  // 編集用の行情報
  const [editingRow, setEditingRow] = useState<FlatRow | null>(null);

  // 編集ダイアログを開く（行単位）
  const handleEditClick = (v: application.VariableDTO) => {
    setEditingVariable(v);
    // 構造体・配列はディープコピーして編集用データを作る
    if (v.dataType.startsWith('ARRAY[') || isStructType(v.dataType)) {
      setEditData(JSON.parse(JSON.stringify(v.value ?? null)));
    } else {
      setEditData(v.value);
    }
    setEditingRow(null);

    setIsEditDialogOpen(true);
  };

  // フラット行から個別の値を編集
  const handleRowEditClick = (row: FlatRow) => {
    if (row.isHeader) {
      // ヘッダー行の場合は変数全体の編集
      handleEditClick(row.variable);
      return;
    }
    setEditingVariable(row.variable);
    setEditingRow(row);
    setEditData(row.value != null ? JSON.parse(JSON.stringify(row.value)) : row.value);

    setIsEditDialogOpen(true);
  };

  // 行単位の値更新
  const handleUpdateRow = async () => {
    if (!editingVariable) return;
    try {
      if (editingRow && editingRow.valuePath.length > 0) {
        // パスをたどって変数全体の値のコピー内の該当箇所を更新
        const fullValue = JSON.parse(JSON.stringify(editingVariable.value));
        let target = fullValue;
        for (let i = 0; i < editingRow.valuePath.length - 1; i++) {
          target = target[editingRow.valuePath[i]];
        }
        target[editingRow.valuePath[editingRow.valuePath.length - 1]] = editData;
        await UpdateVariableValue(editingVariable.id, fullValue);
      } else {
        await UpdateVariableValue(editingVariable.id, editData);
      }
      await loadVariables();
      setIsEditDialogOpen(false);
      setEditingVariable(null);
      setEditingRow(null);
      setEditData(null);
    } catch (e) {
      console.error('Failed to update variable:', e);
      alert('変数の更新に失敗しました: ' + e);
    }
  };

  // 数値入力のパース（接頭辞で自動判定）
  const parseNumericInput = (input: string): number => {
    const trimmed = input.trim();
    if (trimmed.startsWith('0x') || trimmed.startsWith('0X')) {
      return parseInt(trimmed, 16);
    }
    if (trimmed.startsWith('0b') || trimmed.startsWith('0B')) {
      return parseInt(trimmed.slice(2), 2);
    }
    return parseFloat(trimmed) || 0;
  };

  // 数値フォーマット
  const formatNumeric = (value: number, format: 'dec' | 'hex' | 'bin'): string => {
    switch (format) {
      case 'hex': return '0x' + (value >>> 0).toString(16).toUpperCase();
      case 'bin': return '0b' + (value >>> 0).toString(2);
      default: return String(value);
    }
  };

  // スカラー値エディタ
  const renderScalarEditor = (dataType: string, value: any, onChange: (v: any) => void) => {
    if (dataType === 'BOOL') {
      return (
        <select
          value={value ? 'true' : 'false'}
          onChange={(e) => onChange(e.target.value === 'true')}
          style={{ flex: 1 }}
        >
          <option value="true">TRUE</option>
          <option value="false">FALSE</option>
        </select>
      );
    }
    if (dataType === 'STRING' || dataType.startsWith('STRING[')) {
      const maxLen = dataType.startsWith('STRING[') ? parseInt(dataType.match(/\[(\d+)\]/)?.[1] || '0') : 0;
      return (
        <input
          type="text"
          value={value ?? ''}
          onChange={(e) => onChange(maxLen > 0 ? e.target.value.slice(0, maxLen) : e.target.value)}
          style={{ flex: 1 }}
          maxLength={maxLen > 0 ? maxLen : undefined}
          placeholder={maxLen > 0 ? `最大${maxLen}バイト` : undefined}
        />
      );
    }
    // 時間・日付型（文字列として編集）
    if (dataType === 'TIME' || dataType === 'DATE' || dataType === 'TIME_OF_DAY' || dataType === 'DATE_AND_TIME') {
      const placeholders: { [key: string]: string } = {
        'TIME': 'T#1s, T#100ms, T#1h30m',
        'DATE': 'D#2024-01-01',
        'TIME_OF_DAY': 'TOD#12:30:15',
        'DATE_AND_TIME': 'DT#2024-01-01-12:30:15'
      };
      return (
        <input
          type="text"
          value={value ?? ''}
          onChange={(e) => onChange(e.target.value)}
          style={{ flex: 1 }}
          placeholder={placeholders[dataType] || ''}
        />
      );
    }
    // 数値型
    return (
      <input
        type="text"
        value={value ?? 0}
        onChange={(e) => {
          const parsed = parseNumericInput(e.target.value);
          if (!isNaN(parsed)) {
            onChange(parsed);
          }
        }}
        onBlur={(e) => {
          const parsed = parseNumericInput(e.target.value);
          if (!isNaN(parsed)) {
            onChange(parsed);
          }
        }}
        style={{ flex: 1 }}
        placeholder="10進, 0x(16進), 0b(2進)"
      />
    );
  };

  // 再帰的値エディタ
  const renderValueEditor = (dataType: string, value: any, onChange: (v: any) => void, depth: number = 0): JSX.Element => {
    const indent = depth * 16;

    // 配列型
    if (dataType.startsWith('ARRAY[')) {
      const match = dataType.match(/^ARRAY\[(.+);(\d+)\]$/);
      if (!match || !Array.isArray(value)) {
        return <span>-</span>;
      }
      const elemType = match[1];
      const elemIsStruct = isStructType(elemType);

      return (
        <div style={{ marginLeft: indent }}>
          {value.map((elem: any, i: number) => (
            <div key={i} style={{ display: 'flex', gap: '4px', alignItems: 'center', marginBottom: '2px' }}>
              <span style={{ fontSize: '0.8em', color: '#888', minWidth: '30px' }}>[{i}]</span>
              {elemIsStruct ? (
                <StructArrayElementEditor
                  elemType={elemType}
                  elem={elem}
                  onChange={(newElem) => {
                    const newArr = [...value];
                    newArr[i] = newElem;
                    onChange(newArr);
                  }}
                  renderEditor={renderValueEditor}
                />
              ) : (
                renderScalarEditor(elemType, elem, (newVal) => {
                  const newArr = [...value];
                  newArr[i] = newVal;
                  onChange(newArr);
                })
              )}
            </div>
          ))}
        </div>
      );
    }

    // 構造体型
    if (isStructType(dataType)) {
      const st = structTypes.find(s => s.name === dataType);
      if (!st || typeof value !== 'object' || value === null) {
        return <span>-</span>;
      }
      return (
        <div style={{ marginLeft: indent }}>
          {st.fields.map((field) => (
            <div key={field.name} style={{ marginBottom: '4px' }}>
              <div style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
                <span style={{ fontSize: '0.85em', fontWeight: 'bold', minWidth: '80px' }}>
                  {field.name}
                </span>
                <span style={{ fontSize: '0.75em', color: '#888' }}>({field.dataType})</span>
              </div>
              <div style={{ marginLeft: '8px' }}>
                {renderValueEditor(field.dataType, value[field.name], (newVal) => {
                  onChange({ ...value, [field.name]: newVal });
                }, depth + 1)}
              </div>
            </div>
          ))}
        </div>
      );
    }

    // スカラー型
    return renderScalarEditor(dataType, value, onChange);
  };

  // 変数を削除
  const handleDeleteVariable = async (id: string, name: string) => {
    if (!confirm(`変数 "${name}" を削除しますか?`)) return;
    try {
      await DeleteVariable(id);
      await loadVariables();
    } catch (e) {
      console.error('Failed to delete variable:', e);
      alert('変数の削除に失敗しました: ' + e);
    }
  };

  // マッピングダイアログを開く
  const handleMappingClick = (v: application.VariableDTO) => {
    setMappingVariable(v);
    setEditMappings(v.mappings ? [...v.mappings] : []);
    setIsMappingDialogOpen(true);
  };

  // マッピングを追加
  const handleAddMapping = () => {
    setEditMappings([...editMappings, {
      protocolType: protocols.length > 0 ? protocols[0].type : '',
      memoryArea: memoryAreas.length > 0 ? memoryAreas[0].id : '',
      address: 0,
      endianness: 'big',
    } as application.ProtocolMappingDTO]);
  };

  // マッピングを削除
  const handleRemoveMapping = (index: number) => {
    setEditMappings(editMappings.filter((_, i) => i !== index));
  };

  // マッピングを保存
  const handleSaveMappings = async () => {
    if (!mappingVariable) return;
    try {
      await UpdateVariableMappings(mappingVariable.id, editMappings);
      await loadVariables();
      setIsMappingDialogOpen(false);
      setMappingVariable(null);
    } catch (e) {
      console.error('Failed to update mappings:', e);
      alert('マッピングの更新に失敗しました: ' + e);
    }
  };

  return (
    <div className="opcua-variable-view">
      <div className="opcua-toolbar">
        <button onClick={() => setIsAddDialogOpen(true)} className="btn-primary">
          変数を追加
        </button>
        <button onClick={() => setIsStructTypeDialogOpen(true)} className="btn-secondary">
          構造体型管理
        </button>
        <button onClick={loadVariables} className="btn-secondary">
          更新
        </button>
      </div>

      <table className="opcua-variable-table">
        <thead>
          <tr>
            <th>名前</th>
            <th>データ型</th>
            <th>値</th>
            <th>マッピング</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {flatRows.map((row) => (
            <tr
              key={row.key}
              style={{
                backgroundColor: row.isHeader ? 'rgba(255,255,255,0.03)' : undefined,
                fontWeight: row.depth === 0 && row.isHeader ? 'bold' : undefined,
              }}
            >
              <td className="var-name" style={{ paddingLeft: `${8 + row.depth * 16}px` }}>
                {row.isHeader ? (
                  <span
                    onClick={(e) => { e.stopPropagation(); toggleExpand(row.key); }}
                    style={{ cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
                  >
                    <span style={{ fontSize: '0.7em', width: '12px', display: 'inline-block' }}>
                      {expandedRows.has(row.key) ? '\u25BC' : '\u25B6'}
                    </span>
                    {row.displayName}
                  </span>
                ) : (
                  <span style={{ paddingLeft: '16px' }}>{row.displayName}</span>
                )}
              </td>
              <td className="var-type" style={{ fontSize: row.isHeader ? undefined : '0.85em' }}>
                {row.dataType}
              </td>
              <td
                className="var-value"
                onClick={() => handleRowEditClick(row)}
                style={{ cursor: 'pointer' }}
              >
                {row.isHeader ? (
                  <span style={{ color: '#888', fontSize: '0.85em' }}>
                    {row.dataType.startsWith('ARRAY[') && Array.isArray(row.value)
                      ? `(${row.value.length} 要素)`
                      : `{${row.dataType}}`}
                  </span>
                ) : (
                  <span>{formatScalarValue(row.value, row.dataType)}</span>
                )}
              </td>
              <td className="var-mapping">
                {row.depth === 0 ? (
                  <span onClick={() => handleMappingClick(row.variable)} style={{ cursor: 'pointer' }}>
                    {formatMappings(row.variable.mappings)}
                  </span>
                ) : (
                  <span style={{ fontSize: '0.85em', color: '#aaa' }}>
                    {formatMappingsWithOffset(row.variable.mappings, row.wordOffset)}
                  </span>
                )}
              </td>
              <td className="var-actions">
                {row.depth === 0 && (
                  <>
                    {row.isHeader && (
                      <button onClick={() => handleEditClick(row.variable)} className="btn-small btn-secondary">
                        一括編集
                      </button>
                    )}
                    <button onClick={() => handleMappingClick(row.variable)} className="btn-small btn-secondary">
                      マッピング
                    </button>
                    <button onClick={() => handleDeleteVariable(row.variable.id, row.variable.name)} className="btn-small btn-danger">
                      削除
                    </button>
                  </>
                )}
              </td>
            </tr>
          ))}
          {variables.length === 0 && (
            <tr>
              <td colSpan={5} className="empty-message">
                変数がありません。「変数を追加」ボタンで変数を作成してください。
              </td>
            </tr>
          )}
        </tbody>
      </table>

      {/* 変数追加ダイアログ */}
      {isAddDialogOpen && (
        <div className="dialog-overlay">
          <div className="dialog">
            <h3>変数を追加</h3>
            <div className="dialog-content">
              <div className="dialog-row">
                <label>変数名:</label>
                <input
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="例: Motor_Speed"
                  onKeyDown={(e) => { if (e.key === 'Enter') handleAddVariable(); }}
                  autoFocus
                />
              </div>
              <div className="dialog-row">
                <label>型カテゴリ:</label>
                <select
                  value={newTypeCategory}
                  onChange={(e) => {
                    const cat = e.target.value as 'scalar' | 'array' | 'struct';
                    setNewTypeCategory(cat);
                    if (cat === 'scalar') setNewDataType('INT');
                    else if (cat === 'struct' && structTypes.length > 0) setNewDataType(structTypes[0].name);
                  }}
                >
                  <option value="scalar">スカラー型</option>
                  <option value="array">配列型</option>
                  {structTypes.length > 0 && <option value="struct">構造体型</option>}
                </select>
              </div>

              {newTypeCategory === 'scalar' && (
                <>
                  <div className="dialog-row">
                    <label>データ型:</label>
                    <select
                      value={newDataType}
                      onChange={(e) => {
                        setNewDataType(e.target.value);
                        setNewValue(getDefaultValue(e.target.value));
                      }}
                    >
                      {dataTypes.map((t) => (
                        <option key={t.id} value={t.id} title={t.description}>
                          {t.displayName} ({t.description})
                        </option>
                      ))}
                      <option value="STRING">STRING (文字列)</option>
                    </select>
                  </div>
                  {newDataType === 'STRING' && (
                    <div className="dialog-row">
                      <label>バイト長:</label>
                      <input
                        type="number"
                        value={newStringLength}
                        onChange={(e) => setNewStringLength(parseInt(e.target.value) || 1)}
                        min={1}
                        max={256}
                      />
                    </div>
                  )}
                  <div className="dialog-row">
                    <label>初期値:</label>
                    <input
                      type="text"
                      value={newValue}
                      onChange={(e) => setNewValue(e.target.value)}
                      placeholder={getDefaultValue(newDataType)}
                    />
                  </div>
                </>
              )}

              {newTypeCategory === 'array' && (
                <>
                  <div className="dialog-row">
                    <label>要素型:</label>
                    <select
                      value={newArrayElemType}
                      onChange={(e) => setNewArrayElemType(e.target.value)}
                    >
                      <optgroup label="スカラー型">
                        {dataTypes.map((t) => (
                          <option key={t.id} value={t.id}>
                            {t.displayName}
                          </option>
                        ))}
                        <option value="STRING">STRING</option>
                      </optgroup>
                      {structTypes.length > 0 && (
                        <optgroup label="構造体型">
                          {structTypes.map((st) => (
                            <option key={st.name} value={st.name}>
                              {st.name} ({st.wordCount}W)
                            </option>
                          ))}
                        </optgroup>
                      )}
                    </select>
                  </div>
                  {newArrayElemType === 'STRING' && (
                    <div className="dialog-row">
                      <label>バイト長:</label>
                      <input
                        type="number"
                        value={newStringLength}
                        onChange={(e) => setNewStringLength(parseInt(e.target.value) || 1)}
                        min={1}
                        max={256}
                      />
                    </div>
                  )}
                  <div className="dialog-row">
                    <label>要素数:</label>
                    <input
                      type="number"
                      value={newArraySize}
                      onChange={(e) => setNewArraySize(parseInt(e.target.value) || 1)}
                      min={1}
                      max={1000}
                    />
                  </div>
                </>
              )}

              {newTypeCategory === 'struct' && (
                <div className="dialog-row">
                  <label>構造体型:</label>
                  <select
                    value={newDataType}
                    onChange={(e) => setNewDataType(e.target.value)}
                  >
                    {structTypes.map((st) => (
                      <option key={st.name} value={st.name}>
                        {st.name} ({st.wordCount}ワード)
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </div>
            <div className="dialog-buttons">
              <button onClick={() => setIsAddDialogOpen(false)} className="btn-secondary">キャンセル</button>
              <button onClick={handleAddVariable} className="btn-primary">追加</button>
            </div>
          </div>
        </div>
      )}

      {/* 変数編集ダイアログ */}
      {isEditDialogOpen && editingVariable && (
        <div className="dialog-overlay">
          <div className="dialog" style={{ minWidth: '450px', maxHeight: '80vh', display: 'flex', flexDirection: 'column' }}>
            <h3>値を編集</h3>
            <div className="dialog-content" style={{ flex: 1, overflowY: 'auto' }}>
              <div className="dialog-row">
                <label>変数名:</label>
                <span className="dialog-value">
                  {editingRow ? editingRow.displayName : editingVariable.name}
                </span>
              </div>
              <div className="dialog-row">
                <label>データ型:</label>
                <span className="dialog-value">
                  {editingRow ? editingRow.dataType : editingVariable.dataType}
                </span>
              </div>
              <div className="dialog-row" style={{ flexDirection: 'column', alignItems: 'flex-start' }}>
                <label>値:</label>
                <div style={{ width: '100%' }}>
                  {editingRow
                    ? renderValueEditor(editingRow.dataType, editData, setEditData)
                    : renderValueEditor(editingVariable.dataType, editData, setEditData)
                  }
                </div>
              </div>
            </div>
            <div className="dialog-buttons">
              <button onClick={() => { setIsEditDialogOpen(false); setEditData(null); setEditingRow(null); }} className="btn-secondary">キャンセル</button>
              <button onClick={handleUpdateRow} className="btn-primary">更新</button>
            </div>
          </div>
        </div>
      )}

      {/* マッピング編集ダイアログ */}
      {isMappingDialogOpen && mappingVariable && (
        <div className="dialog-overlay">
          <div className="dialog" style={{ minWidth: '500px' }}>
            <h3>マッピング設定: {mappingVariable.name}</h3>
            <div className="dialog-content">
              <p className="dialog-description">
                この変数をプロトコルのアドレスにマッピングします。
              </p>

              {editMappings.map((m, index) => (
                <div key={index} className="dialog-section">
                  {/* ヘッダ行 */}
                  <div className="dialog-row">
                    <label style={{ flex: 1 }}>プロトコル</label>
                    <label style={{ flex: 1 }}>メモリエリア</label>
                    <label style={{ flex: 1 }}>アドレス</label>
                    <label style={{ flex: 1 }}>エンディアン</label>
                    <span style={{ width: '60px' }}></span>
                  </div>
                  {/* コントロール行 */}
                  <div className="dialog-row">
                    <select
                      value={m.protocolType}
                      onChange={(e) => {
                        const updated = [...editMappings];
                        updated[index] = { ...updated[index], protocolType: e.target.value };
                        setEditMappings(updated);
                      }}
                      style={{ flex: 1 }}
                    >
                      {protocols.map((p) => (
                        <option key={p.type} value={p.type}>{p.displayName}</option>
                      ))}
                    </select>

                    <select
                      value={m.memoryArea}
                      onChange={(e) => {
                        const updated = [...editMappings];
                        updated[index] = { ...updated[index], memoryArea: e.target.value };
                        setEditMappings(updated);
                      }}
                      style={{ flex: 1 }}
                    >
                      {memoryAreas.map((a) => (
                        <option key={a.id} value={a.id}>{a.displayName}</option>
                      ))}
                    </select>

                    <input
                      type="number"
                      value={m.address}
                      onChange={(e) => {
                        const updated = [...editMappings];
                        updated[index] = { ...updated[index], address: parseInt(e.target.value) || 0 };
                        setEditMappings(updated);
                      }}
                      min={0}
                      style={{ flex: 1 }}
                    />

                    <select
                      value={m.endianness}
                      onChange={(e) => {
                        const updated = [...editMappings];
                        updated[index] = { ...updated[index], endianness: e.target.value };
                        setEditMappings(updated);
                      }}
                      style={{ flex: 1 }}
                    >
                      <option value="big">Big</option>
                      <option value="little">Little</option>
                    </select>

                    <button
                      onClick={() => handleRemoveMapping(index)}
                      className="btn-small btn-danger"
                    >
                      X
                    </button>
                  </div>
                </div>
              ))}

              <button onClick={handleAddMapping} className="btn-secondary">
                + マッピング追加
              </button>
            </div>
            <div className="dialog-buttons">
              <button onClick={() => setIsMappingDialogOpen(false)} className="btn-secondary">キャンセル</button>
              <button onClick={handleSaveMappings} className="btn-primary">保存</button>
            </div>
          </div>
        </div>
      )}
      {/* 構造体型管理ダイアログ */}
      {isStructTypeDialogOpen && (
        <div className="dialog-overlay">
          <div className="dialog" style={{ minWidth: '500px', maxHeight: '80vh', display: 'flex', flexDirection: 'column' }}>
            <h3>構造体型管理</h3>
            <div className="dialog-content" style={{ flex: 1, overflowY: 'auto' }}>
              {/* 既存の構造体型一覧 */}
              {structTypes.length > 0 && (
                <div className="dialog-section">
                  <h4 className="dialog-section-title">登録済み構造体型</h4>
                  <table className="opcua-variable-table">
                    <thead>
                      <tr>
                        <th>名前</th>
                        <th>フィールド</th>
                        <th>ワード数</th>
                        <th>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {structTypes.map((st) => (
                        <tr key={st.name}>
                          <td>{st.name}</td>
                          <td>{st.fields.map(f => `${f.name}:${f.dataType}`).join(', ')}</td>
                          <td>{st.wordCount}</td>
                          <td>
                            <button onClick={() => handleEditStructType(st)} className="btn-small btn-secondary" style={{ marginRight: '0.5rem' }}>
                              編集
                            </button>
                            <button onClick={() => handleDeleteStructType(st.name)} className="btn-small btn-danger">
                              削除
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {/* 新規構造体型登録 */}
              <h4 className="dialog-section-title">{editingStructTypeName ? '構造体型を編集' : '新規構造体型'}</h4>
              <div className="dialog-row">
                <label>型名:</label>
                <input
                  type="text"
                  value={structTypeName}
                  onChange={(e) => setStructTypeName(e.target.value)}
                  placeholder="例: MotorData"
                  disabled={!!editingStructTypeName}
                />
              </div>
              <div className="dialog-section">
                <label>フィールド:</label>
                {structTypeFields.map((field, index) => (
                  <div key={index} className="dialog-field-editor">
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
                      <strong>フィールド {index + 1}</strong>
                      <button
                        onClick={() => setStructTypeFields(structTypeFields.filter((_, i) => i !== index))}
                        className="btn-small btn-danger"
                        disabled={structTypeFields.length <= 1}
                      >
                        X
                      </button>
                    </div>

                    <div className="dialog-row">
                      <label>名前:</label>
                      <input
                        type="text"
                        value={field.name}
                        onChange={(e) => {
                          const updated = [...structTypeFields];
                          updated[index] = { ...updated[index], name: e.target.value };
                          setStructTypeFields(updated);
                        }}
                        placeholder="フィールド名"
                      />
                    </div>

                    <div className="dialog-row">
                      <label>カテゴリ:</label>
                      <select
                        value={field.category}
                        onChange={(e) => {
                          const updated = [...structTypeFields];
                          const cat = e.target.value as 'scalar' | 'struct' | 'array';
                          updated[index] = {
                            ...updated[index],
                            category: cat,
                            dataType: cat === 'struct' ? (structTypes.length > 0 ? structTypes[0].name : '') : 'INT',
                          };
                          setStructTypeFields(updated);
                        }}
                      >
                        <option value="scalar">スカラー</option>
                        {structTypes.length > 0 && <option value="struct">構造体</option>}
                        <option value="array">配列</option>
                      </select>
                    </div>

                    {field.category === 'scalar' && (
                      <div className="dialog-row">
                        <label>データ型:</label>
                        <select
                          value={field.dataType}
                          onChange={(e) => {
                            const updated = [...structTypeFields];
                            updated[index] = { ...updated[index], dataType: e.target.value };
                            setStructTypeFields(updated);
                          }}
                        >
                          {dataTypes.map((t) => (
                            <option key={t.id} value={t.id}>{t.displayName}</option>
                          ))}
                          <option value="STRING">STRING</option>
                        </select>
                        {field.dataType === 'STRING' && (
                          <>
                            <label>バイト長:</label>
                            <input
                              type="number"
                              value={field.stringLength}
                              onChange={(e) => {
                                const updated = [...structTypeFields];
                                updated[index] = { ...updated[index], stringLength: parseInt(e.target.value) || 1 };
                                setStructTypeFields(updated);
                              }}
                              min={1}
                              max={256}
                            />
                          </>
                        )}
                      </div>
                    )}

                    {field.category === 'struct' && (
                      <div className="dialog-row">
                        <label>構造体型:</label>
                        <select
                          value={field.dataType}
                          onChange={(e) => {
                            const updated = [...structTypeFields];
                            updated[index] = { ...updated[index], dataType: e.target.value };
                            setStructTypeFields(updated);
                          }}
                        >
                          {structTypes
                            .filter(st => st.name !== structTypeName.trim())
                            .map((st) => (
                              <option key={st.name} value={st.name}>{st.name} ({st.wordCount}W)</option>
                            ))}
                        </select>
                      </div>
                    )}

                    {field.category === 'array' && (
                      <>
                        <div className="dialog-row">
                          <label>要素カテゴリ:</label>
                          <select
                            value={field.arrayElemCategory}
                            onChange={(e) => {
                              const updated = [...structTypeFields];
                              const elemCat = e.target.value as 'scalar' | 'struct';
                              updated[index] = {
                                ...updated[index],
                                arrayElemCategory: elemCat,
                                arrayElemType: elemCat === 'struct' ? (structTypes.length > 0 ? structTypes[0].name : '') : 'INT',
                              };
                              setStructTypeFields(updated);
                            }}
                          >
                            <option value="scalar">スカラー</option>
                            {structTypes.length > 0 && <option value="struct">構造体</option>}
                          </select>
                        </div>

                        <div className="dialog-row">
                          <label>要素型:</label>
                          <select
                            value={field.arrayElemType}
                            onChange={(e) => {
                              const updated = [...structTypeFields];
                              updated[index] = { ...updated[index], arrayElemType: e.target.value };
                              setStructTypeFields(updated);
                            }}
                          >
                            {field.arrayElemCategory === 'scalar'
                              ? <>
                                  {dataTypes.map((t) => (
                                    <option key={t.id} value={t.id}>{t.displayName}</option>
                                  ))}
                                  <option value="STRING">STRING</option>
                                </>
                              : structTypes
                                  .filter(st => st.name !== structTypeName.trim())
                                  .map((st) => (
                                    <option key={st.name} value={st.name}>{st.name} ({st.wordCount}W)</option>
                                  ))
                            }
                          </select>
                          {field.arrayElemCategory === 'scalar' && field.arrayElemType === 'STRING' && (
                            <>
                              <label>バイト長:</label>
                              <input
                                type="number"
                                value={field.stringLength}
                                onChange={(e) => {
                                  const updated = [...structTypeFields];
                                  updated[index] = { ...updated[index], stringLength: parseInt(e.target.value) || 1 };
                                  setStructTypeFields(updated);
                                }}
                                min={1}
                                max={256}
                              />
                            </>
                          )}
                        </div>

                        <div className="dialog-row">
                          <label>配列サイズ:</label>
                          <input
                            type="number"
                            value={field.arraySize}
                            onChange={(e) => {
                              const updated = [...structTypeFields];
                              updated[index] = { ...updated[index], arraySize: parseInt(e.target.value) || 1 };
                              setStructTypeFields(updated);
                            }}
                            min={1}
                            max={1000}
                          />
                        </div>
                      </>
                    )}
                  </div>
                ))}
                <button
                  onClick={() => setStructTypeFields([...structTypeFields, { name: '', category: 'scalar', dataType: 'INT', stringLength: 20, arrayElemType: 'INT', arrayElemCategory: 'scalar', arraySize: 10 }])}
                  className="btn-secondary"
                  style={{ marginTop: '0.5rem' }}
                >
                  + フィールド追加
                </button>
              </div>
            </div>
            <div className="dialog-buttons">
              <button onClick={() => {
                setIsStructTypeDialogOpen(false);
                setEditingStructTypeName(null);
                setStructTypeName('');
                setStructTypeFields([{ name: '', category: 'scalar', dataType: 'INT', stringLength: 20, arrayElemType: 'INT', arrayElemCategory: 'scalar', arraySize: 10 }]);
              }} className="btn-secondary">閉じる</button>
              <button onClick={handleRegisterStructType} className="btn-primary">{editingStructTypeName ? '更新' : '型を登録'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
