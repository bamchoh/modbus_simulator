import { useState, useEffect, useCallback, useRef } from 'react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragEndEvent
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import {
  GetMonitoringItems,
  AddMonitoringItem,
  UpdateMonitoringItem,
  DeleteMonitoringItem,
  ReorderMonitoringItem,
  ReadWords,
  ReadBits,
  WriteBit,
  WriteWord
} from '../../wailsjs/go/main/App';
import { application } from '../../wailsjs/go/models';

type DisplayFormat = 'decimal' | 'hex' | 'octal' | 'binary';
type BitWidth = 16 | 32 | 64;
type Endianness = 'little' | 'big';

const DISPLAY_FORMATS: { value: DisplayFormat; label: string }[] = [
  { value: 'decimal', label: '10進数' },
  { value: 'hex', label: '16進数' },
  { value: 'octal', label: '8進数' },
  { value: 'binary', label: '2進数' }
];

const BIT_WIDTHS: { value: BitWidth; label: string; wordCount: number }[] = [
  { value: 16, label: '16bit', wordCount: 1 },
  { value: 32, label: '32bit', wordCount: 2 },
  { value: 64, label: '64bit', wordCount: 4 },
];

const ENDIANNESS_OPTIONS: { value: Endianness; label: string }[] = [
  { value: 'big', label: 'BE' },
  { value: 'little', label: 'LE' },
];

// ビット幅に応じたワード数を取得
const getWordCount = (bitWidth: BitWidth): number => {
  return BIT_WIDTHS.find(b => b.value === bitWidth)?.wordCount ?? 1;
};

// 複数ワードを結合して数値に変換
const combineWords = (words: number[], endianness: Endianness): bigint => {
  if (words.length === 0) return BigInt(0);
  const orderedWords = endianness === 'little' ? words : [...words].reverse();
  let result = BigInt(0);
  for (let i = orderedWords.length - 1; i >= 0; i--) {
    result = (result << BigInt(16)) | BigInt(orderedWords[i] & 0xFFFF);
  }
  return result;
};

// bigintを指定形式でフォーマット
const formatBigInt = (value: bigint, format: DisplayFormat, bitWidth: BitWidth): string => {
  const absValue = value < 0 ? -value : value;
  switch (format) {
    case 'hex': {
      const hexDigits = bitWidth / 4;
      return '0x' + absValue.toString(16).toUpperCase().padStart(hexDigits, '0');
    }
    case 'octal':
      return '0o' + absValue.toString(8);
    case 'binary':
      return absValue.toString(2).padStart(bitWidth, '0');
    default:
      return value.toString();
  }
};

// 16bit値をフォーマット
const formatSingleWord = (value: number, format: DisplayFormat): string => {
  switch (format) {
    case 'hex':
      return '0x' + value.toString(16).toUpperCase().padStart(4, '0');
    case 'octal':
      return '0o' + value.toString(8).padStart(6, '0');
    case 'binary':
      return value.toString(2).padStart(16, '0');
    default:
      return value.toString();
  }
};

// bigintを複数ワードに分解
const splitToWords = (value: bigint, wordCount: number, endianness: Endianness): number[] => {
  const words: number[] = [];
  let remaining = value < 0 ? -value : value;
  const mask = BigInt(0xFFFF);
  for (let i = 0; i < wordCount; i++) {
    words.push(Number(remaining & mask));
    remaining = remaining >> BigInt(16);
  }
  return endianness === 'little' ? words : words.reverse();
};

// 文字列入力をbigintにパース
const parseBigIntInput = (input: string, format: DisplayFormat): bigint => {
  const trimmed = input.trim();
  switch (format) {
    case 'hex': {
      const hexStr = trimmed.replace(/^0x/i, '');
      return BigInt('0x' + hexStr);
    }
    case 'octal': {
      const octStr = trimmed.replace(/^0o/i, '');
      return BigInt('0o' + octStr);
    }
    case 'binary': {
      const binStr = trimmed.replace(/^0b/i, '');
      return BigInt('0b' + binStr);
    }
    default:
      return BigInt(trimmed);
  }
};

// 入力値をパース（16bit用）
const parseInputValue = (input: string, format: DisplayFormat): number => {
  const trimmed = input.trim();
  switch (format) {
    case 'hex': {
      const hexStr = trimmed.replace(/^0x/i, '');
      return parseInt(hexStr, 16);
    }
    case 'octal': {
      const octStr = trimmed.replace(/^0o/i, '');
      return parseInt(octStr, 8);
    }
    case 'binary': {
      const binStr = trimmed.replace(/^0b/i, '');
      return parseInt(binStr, 2);
    }
    default:
      return parseInt(trimmed, 10);
  }
};

interface MonitoringItemWithValue {
  item: application.MonitoringItemDTO;
  currentValue: string;
  rawValues: number[];
  isBit: boolean;
  bitValue?: boolean;
}

interface Props {
  memoryAreas: application.MemoryAreaDTO[];
}

// ドラッグ可能な行コンポーネント
interface SortableRowProps {
  itemWithValue: MonitoringItemWithValue;
  memoryAreas: application.MemoryAreaDTO[];
  onSettingChange: (item: MonitoringItemWithValue, field: 'displayFormat' | 'bitWidth' | 'endianness', value: string | number) => void;
  onValueClick: (item: MonitoringItemWithValue) => void;
  onDelete: (id: string) => void;
}

function SortableRow({ itemWithValue, memoryAreas, onSettingChange, onValueClick, onDelete }: SortableRowProps) {
  const item = itemWithValue.item;
  const area = memoryAreas.find(a => a.id === item.memoryArea);

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging
  } = useSortable({ id: item.id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    backgroundColor: isDragging ? 'rgba(233, 69, 96, 0.1)' : undefined
  };

  return (
    <tr ref={setNodeRef} style={style}>
      <td className="monitoring-drag-handle" {...attributes} {...listeners}>
        <span className="drag-handle">⠿</span>
      </td>
      <td>{area?.displayName || item.memoryArea}</td>
      <td>{item.address}</td>
      <td>
        {!itemWithValue.isBit ? (
          <select
            value={item.bitWidth}
            onChange={(e) => onSettingChange(itemWithValue, 'bitWidth', parseInt(e.target.value))}
            className="inline-select"
          >
            {BIT_WIDTHS.map(b => (
              <option key={b.value} value={b.value}>{b.label}</option>
            ))}
          </select>
        ) : (
          'Bit'
        )}
      </td>
      <td>
        {!itemWithValue.isBit ? (
          <select
            value={item.endianness}
            onChange={(e) => onSettingChange(itemWithValue, 'endianness', e.target.value)}
            className="inline-select"
          >
            {ENDIANNESS_OPTIONS.map(e => (
              <option key={e.value} value={e.value}>{e.label}</option>
            ))}
          </select>
        ) : (
          '-'
        )}
      </td>
      <td>
        {!itemWithValue.isBit ? (
          <select
            value={item.displayFormat}
            onChange={(e) => onSettingChange(itemWithValue, 'displayFormat', e.target.value)}
            className="inline-select"
          >
            {DISPLAY_FORMATS.map(f => (
              <option key={f.value} value={f.value}>{f.label}</option>
            ))}
          </select>
        ) : (
          '-'
        )}
      </td>
      <td
        className="monitoring-value"
        onClick={() => onValueClick(itemWithValue)}
      >
        {itemWithValue.currentValue}
      </td>
      <td className="monitoring-actions">
        <button onClick={() => onDelete(item.id)} className="btn-small btn-danger">
          削除
        </button>
      </td>
    </tr>
  );
}

export function MonitoringView({ memoryAreas }: Props) {
  const [items, setItems] = useState<MonitoringItemWithValue[]>([]);
  const [autoRefresh, setAutoRefresh] = useState(true);

  // ドラッグ＆ドロップ用センサー
  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates
    })
  );

  // 追加ダイアログ
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);
  const [formArea, setFormArea] = useState('');
  const [formAddress, setFormAddress] = useState(0);
  const [formCount, setFormCount] = useState(1);
  const [formBitWidth, setFormBitWidth] = useState<BitWidth>(16);
  const [formEndianness, setFormEndianness] = useState<Endianness>('big');
  const [formDisplayFormat, setFormDisplayFormat] = useState<DisplayFormat>('decimal');

  // 書き込みダイアログ
  const [isWriteDialogOpen, setIsWriteDialogOpen] = useState(false);
  const [writingItem, setWritingItem] = useState<MonitoringItemWithValue | null>(null);
  const [writeValue, setWriteValue] = useState('');
  const [writeInputFormat, setWriteInputFormat] = useState<DisplayFormat>('decimal');
  const dialogInputRef = useRef<HTMLInputElement>(null);

  // 項目一覧を読み込み
  const loadItems = useCallback(async () => {
    try {
      const monitoringItems = await GetMonitoringItems();
      if (!monitoringItems || monitoringItems.length === 0) {
        setItems([]);
        return;
      }

      // 各項目の現在値を取得
      const itemsWithValues: MonitoringItemWithValue[] = await Promise.all(
        monitoringItems.map(async (item) => {
          const area = memoryAreas.find(a => a.id === item.memoryArea);
          const isBit = area?.isBit ?? false;
          const bitWidth = (item.bitWidth as BitWidth) || 16;
          const endianness = (item.endianness as Endianness) || 'big';
          const displayFormat = (item.displayFormat as DisplayFormat) || 'decimal';

          try {
            if (isBit) {
              const bits = await ReadBits(item.memoryArea, item.address, 1);
              const bitValue = bits?.[0] ?? false;
              return {
                item,
                currentValue: bitValue ? 'ON' : 'OFF',
                rawValues: [],
                isBit: true,
                bitValue
              };
            } else {
              const wordCount = getWordCount(bitWidth);
              const words = await ReadWords(item.memoryArea, item.address, wordCount);
              if (!words || words.length < wordCount) {
                return {
                  item,
                  currentValue: '---',
                  rawValues: [],
                  isBit: false
                };
              }

              let formattedValue: string;
              if (bitWidth === 16) {
                formattedValue = formatSingleWord(words[0], displayFormat);
              } else {
                const combined = combineWords(words, endianness);
                formattedValue = formatBigInt(combined, displayFormat, bitWidth);
              }

              return {
                item,
                currentValue: formattedValue,
                rawValues: words,
                isBit: false
              };
            }
          } catch {
            return {
              item,
              currentValue: 'Error',
              rawValues: [],
              isBit
            };
          }
        })
      );

      setItems(itemsWithValues);
    } catch (e) {
      console.error('Failed to load monitoring items:', e);
    }
  }, [memoryAreas]);

  // 初回読み込み
  useEffect(() => {
    loadItems();
  }, [loadItems]);

  // 自動更新
  useEffect(() => {
    if (autoRefresh) {
      const interval = setInterval(loadItems, 100);
      return () => clearInterval(interval);
    }
  }, [autoRefresh, loadItems]);

  // ダイアログが開いた時に入力値を全選択
  useEffect(() => {
    if (isWriteDialogOpen && dialogInputRef.current) {
      dialogInputRef.current.focus();
      dialogInputRef.current.select();
    }
  }, [isWriteDialogOpen]);

  // 追加ダイアログを開く
  const handleAdd = () => {
    setFormArea(memoryAreas.find(a => !a.isBit)?.id || memoryAreas[0]?.id || '');
    setFormAddress(0);
    setFormCount(1);
    setFormBitWidth(16);
    setFormEndianness('big');
    setFormDisplayFormat('decimal');
    setIsAddDialogOpen(true);
  };

  // 保存（複数追加対応）
  const handleSave = async () => {
    try {
      for (let i = 0; i < formCount; i++) {
        const area = memoryAreas.find(a => a.id === formArea);
        const isBit = area?.isBit ?? false;
        const addressIncrement = isBit ? 1 : getWordCount(formBitWidth);

        const itemData: application.MonitoringItemDTO = {
          id: '',
          order: 0,
          memoryArea: formArea,
          address: formAddress + i * addressIncrement,
          bitWidth: formBitWidth,
          endianness: formEndianness,
          displayFormat: formDisplayFormat
        };

        await AddMonitoringItem(itemData);
      }

      setIsAddDialogOpen(false);
      await loadItems();
    } catch (e) {
      console.error('Failed to save monitoring item:', e);
    }
  };

  // 設定変更（表示形式、ビット幅、エンディアン）
  const handleSettingChange = async (
    itemWithValue: MonitoringItemWithValue,
    field: 'displayFormat' | 'bitWidth' | 'endianness',
    value: string | number
  ) => {
    const item = { ...itemWithValue.item };
    if (field === 'displayFormat') {
      item.displayFormat = value as string;
    } else if (field === 'bitWidth') {
      item.bitWidth = value as number;
    } else if (field === 'endianness') {
      item.endianness = value as string;
    }

    try {
      await UpdateMonitoringItem(item);
      await loadItems();
    } catch (e) {
      console.error('Failed to update monitoring item:', e);
    }
  };

  // 削除
  const handleDelete = async (id: string) => {
    try {
      await DeleteMonitoringItem(id);
      await loadItems();
    } catch (e) {
      console.error('Failed to delete monitoring item:', e);
    }
  };

  // ドラッグ＆ドロップによる並び替え
  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    if (over && active.id !== over.id) {
      const oldIndex = items.findIndex(item => item.item.id === active.id);
      const newIndex = items.findIndex(item => item.item.id === over.id);

      // UIを先に更新（楽観的更新）
      setItems((items) => arrayMove(items, oldIndex, newIndex));

      // バックエンドに通知
      try {
        await ReorderMonitoringItem(active.id as string, newIndex);
      } catch (e) {
        console.error('Failed to reorder monitoring item:', e);
        // エラー時はリロード
        await loadItems();
      }
    }
  };

  // 書き込みダイアログを開く
  const handleValueClick = (itemWithValue: MonitoringItemWithValue) => {
    setWritingItem(itemWithValue);
    setWriteInputFormat((itemWithValue.item.displayFormat as DisplayFormat) || 'decimal');
    setWriteValue(itemWithValue.currentValue);
    setIsWriteDialogOpen(true);
  };

  // 書き込みダイアログを閉じる
  const handleWriteDialogClose = () => {
    setIsWriteDialogOpen(false);
  };

  // 入力形式変更時に値を変換
  const handleWriteInputFormatChange = (newFormat: DisplayFormat) => {
    if (writingItem && !writingItem.isBit && writeValue) {
      try {
        const bitWidth = (writingItem.item.bitWidth as BitWidth) || 16;
        if (bitWidth === 16) {
          const numValue = parseInputValue(writeValue, writeInputFormat);
          if (!isNaN(numValue)) {
            setWriteValue(formatSingleWord(numValue, newFormat));
          }
        } else {
          const bigValue = parseBigIntInput(writeValue, writeInputFormat);
          setWriteValue(formatBigInt(bigValue, newFormat, bitWidth));
        }
      } catch {
        // パース失敗時は値をそのまま
      }
    }
    setWriteInputFormat(newFormat);
  };

  // 書き込み実行
  const handleWrite = async () => {
    if (!writingItem) return;

    try {
      const item = writingItem.item;
      const area = memoryAreas.find(a => a.id === item.memoryArea);
      const isBit = area?.isBit ?? false;

      if (isBit) {
        const newValue = writeValue === 'true' || writeValue === '1' || writeValue.toLowerCase() === 'on';
        await WriteBit(item.memoryArea, item.address, newValue);
      } else {
        const bitWidth = (item.bitWidth as BitWidth) || 16;
        const endianness = (item.endianness as Endianness) || 'big';

        if (bitWidth === 16) {
          const newValue = parseInputValue(writeValue, writeInputFormat);
          if (isNaN(newValue)) {
            console.error('Invalid number format');
            return;
          }
          await WriteWord(item.memoryArea, item.address, newValue);
        } else {
          const bigValue = parseBigIntInput(writeValue, writeInputFormat);
          const wordCount = getWordCount(bitWidth);
          const words = splitToWords(bigValue, wordCount, endianness);
          for (let i = 0; i < words.length; i++) {
            await WriteWord(item.memoryArea, item.address + i, words[i]);
          }
        }
      }

      setIsWriteDialogOpen(false);
      await loadItems();
    } catch (e) {
      console.error('Failed to write value:', e);
    }
  };

  // 書き込みダイアログ内のキーハンドラ
  const handleWriteDialogKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleWrite();
    } else if (e.key === 'Escape') {
      handleWriteDialogClose();
    }
  };

  // 選択されたエリアがビットタイプかどうか
  const selectedAreaIsBit = memoryAreas.find(a => a.id === formArea)?.isBit ?? false;

  return (
    <div className="monitoring-view">
      <div className="monitoring-controls">
        <button onClick={handleAdd} className="btn-primary">
          追加
        </button>
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={autoRefresh}
            onChange={(e) => setAutoRefresh(e.target.checked)}
          />
          自動更新
        </label>
        <button onClick={loadItems} className="btn-secondary">
          更新
        </button>
      </div>

      {items.length === 0 ? (
        <div className="monitoring-empty">
          モニタリング項目がありません。「追加」ボタンで項目を登録してください。
        </div>
      ) : (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={handleDragEnd}
        >
          <table className="monitoring-table">
            <thead>
              <tr>
                <th></th>
                <th>エリア</th>
                <th>アドレス</th>
                <th>ビット幅</th>
                <th>エンディアン</th>
                <th>表示形式</th>
                <th>現在値</th>
                <th>操作</th>
              </tr>
            </thead>
            <SortableContext
              items={items.map(i => i.item.id)}
              strategy={verticalListSortingStrategy}
            >
              <tbody>
                {items.map((itemWithValue) => (
                  <SortableRow
                    key={itemWithValue.item.id}
                    itemWithValue={itemWithValue}
                    memoryAreas={memoryAreas}
                    onSettingChange={handleSettingChange}
                    onValueClick={handleValueClick}
                    onDelete={handleDelete}
                  />
                ))}
              </tbody>
            </SortableContext>
          </table>
        </DndContext>
      )}

      {/* 追加ダイアログ */}
      {isAddDialogOpen && (
        <div className="dialog-overlay">
          <div className="dialog">
            <h3>モニタリング項目を追加</h3>

            <div className="dialog-content">
              <div className="dialog-row">
                <label>メモリエリア:</label>
                <select value={formArea} onChange={(e) => setFormArea(e.target.value)}>
                  {memoryAreas.map(area => (
                    <option key={area.id} value={area.id}>{area.displayName}</option>
                  ))}
                </select>
              </div>

              <div className="dialog-row">
                <label>開始アドレス:</label>
                <input
                  type="number"
                  min="0"
                  max="65535"
                  value={formAddress}
                  onChange={(e) => setFormAddress(parseInt(e.target.value) || 0)}
                />
              </div>

              <div className="dialog-row">
                <label>個数:</label>
                <input
                  type="number"
                  min="1"
                  max="100"
                  value={formCount}
                  onChange={(e) => setFormCount(parseInt(e.target.value) || 1)}
                />
              </div>

              {!selectedAreaIsBit && (
                <>
                  <div className="dialog-row">
                    <label>ビット幅:</label>
                    <select
                      value={formBitWidth}
                      onChange={(e) => setFormBitWidth(parseInt(e.target.value) as BitWidth)}
                    >
                      {BIT_WIDTHS.map(b => (
                        <option key={b.value} value={b.value}>{b.label}</option>
                      ))}
                    </select>
                  </div>

                  <div className="dialog-row">
                    <label>エンディアン:</label>
                    <select
                      value={formEndianness}
                      onChange={(e) => setFormEndianness(e.target.value as Endianness)}
                    >
                      {ENDIANNESS_OPTIONS.map(e => (
                        <option key={e.value} value={e.value}>{e.label}</option>
                      ))}
                    </select>
                  </div>

                  <div className="dialog-row">
                    <label>表示形式:</label>
                    <select
                      value={formDisplayFormat}
                      onChange={(e) => setFormDisplayFormat(e.target.value as DisplayFormat)}
                    >
                      {DISPLAY_FORMATS.map(f => (
                        <option key={f.value} value={f.value}>{f.label}</option>
                      ))}
                    </select>
                  </div>
                </>
              )}
            </div>

            <div className="dialog-buttons">
              <button onClick={() => setIsAddDialogOpen(false)} className="btn-secondary">
                キャンセル
              </button>
              <button onClick={handleSave} className="btn-primary">
                追加
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 書き込みダイアログ */}
      {isWriteDialogOpen && writingItem && (
        <div className="dialog-overlay">
          <div className="dialog">
            <h3>レジスタ書き込み</h3>

            <div className="dialog-content">
              <div className="dialog-row">
                <label>アドレス:</label>
                <span className="dialog-value">{writingItem.item.address}</span>
              </div>

              <div className="dialog-row">
                <label>現在の値:</label>
                <span className="dialog-value">{writingItem.currentValue}</span>
              </div>

              {!writingItem.isBit && (
                <div className="dialog-row">
                  <label>入力形式:</label>
                  <select
                    value={writeInputFormat}
                    onChange={(e) => handleWriteInputFormatChange(e.target.value as DisplayFormat)}
                  >
                    {DISPLAY_FORMATS.map(f => (
                      <option key={f.value} value={f.value}>{f.label}</option>
                    ))}
                  </select>
                </div>
              )}

              <div className="dialog-row">
                <label>新しい値:</label>
                <input
                  ref={dialogInputRef}
                  type="text"
                  value={writeValue}
                  onChange={(e) => setWriteValue(e.target.value)}
                  onKeyDown={handleWriteDialogKeyDown}
                  className="dialog-input"
                  placeholder={writingItem.isBit ? '0, 1, ON, OFF' : ''}
                />
              </div>
            </div>

            <div className="dialog-buttons">
              <button onClick={handleWriteDialogClose} className="btn-secondary">
                キャンセル
              </button>
              <button onClick={handleWrite} className="btn-primary">
                書き込み
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
