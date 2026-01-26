import { useState, useEffect, useCallback } from 'react';
import {
  GetCoils,
  GetDiscreteInputs,
  GetHoldingRegisters,
  GetInputRegisters,
  SetCoil,
  SetDiscreteInput,
  SetHoldingRegister,
  SetInputRegister
} from '../../wailsjs/go/main/App';

type RegisterType = 'coils' | 'discreteInputs' | 'holdingRegisters' | 'inputRegisters';
type DisplayFormat = 'decimal' | 'hex' | 'octal' | 'binary';

const REGISTER_TYPES: { value: RegisterType; label: string; isBool: boolean }[] = [
  { value: 'coils', label: 'コイル (0x)', isBool: true },
  { value: 'discreteInputs', label: 'ディスクリート入力 (1x)', isBool: true },
  { value: 'holdingRegisters', label: '保持レジスタ (4x)', isBool: false },
  { value: 'inputRegisters', label: '入力レジスタ (3x)', isBool: false }
];

const DISPLAY_FORMATS: { value: DisplayFormat; label: string }[] = [
  { value: 'decimal', label: '10進数' },
  { value: 'hex', label: '16進数' },
  { value: 'octal', label: '8進数' },
  { value: 'binary', label: '2進数' }
];

const COLUMNS = 10;
const PAGE_SIZE = 100;

export function RegisterPanel() {
  const [registerType, setRegisterType] = useState<RegisterType>('holdingRegisters');
  const [startAddress, setStartAddress] = useState(0);
  const [values, setValues] = useState<(boolean | number)[]>([]);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [displayFormat, setDisplayFormat] = useState<DisplayFormat>('decimal');

  // モーダルダイアログの状態
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingAddress, setEditingAddress] = useState<number>(0);
  const [editingOriginalValue, setEditingOriginalValue] = useState<boolean | number>(0);
  const [editInputFormat, setEditInputFormat] = useState<DisplayFormat>('decimal');
  const [editValue, setEditValue] = useState('');

  const loadRegisters = useCallback(async () => {
    try {
      let data: (boolean | number)[];
      switch (registerType) {
        case 'coils':
          data = await GetCoils(startAddress, PAGE_SIZE);
          break;
        case 'discreteInputs':
          data = await GetDiscreteInputs(startAddress, PAGE_SIZE);
          break;
        case 'holdingRegisters':
          data = await GetHoldingRegisters(startAddress, PAGE_SIZE);
          break;
        case 'inputRegisters':
          data = await GetInputRegisters(startAddress, PAGE_SIZE);
          break;
      }
      setValues(data || []);
    } catch (e) {
      console.error('Failed to load registers:', e);
    }
  }, [registerType, startAddress]);

  useEffect(() => {
    loadRegisters();
  }, [loadRegisters]);

  useEffect(() => {
    if (autoRefresh) {
      const interval = setInterval(loadRegisters, 500);
      return () => clearInterval(interval);
    }
  }, [autoRefresh, loadRegisters]);

  const isBoolType = REGISTER_TYPES.find(t => t.value === registerType)?.isBool ?? false;

  // 指定した形式で値をフォーマット
  const formatValueWithFormat = (value: boolean | number, format: DisplayFormat) => {
    if (typeof value === 'boolean') {
      return value ? 'ON' : 'OFF';
    }
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

  // 表示用フォーマット（現在の表示形式を使用）
  const formatValue = (value: boolean | number) => {
    return formatValueWithFormat(value, displayFormat);
  };

  // 入力値をパース
  const parseInputValue = (input: string, format: DisplayFormat): number => {
    const trimmed = input.trim();
    switch (format) {
      case 'hex':
        // 0x プレフィックスを除去
        const hexStr = trimmed.replace(/^0x/i, '');
        return parseInt(hexStr, 16);
      case 'octal':
        // 0o プレフィックスを除去
        const octStr = trimmed.replace(/^0o/i, '');
        return parseInt(octStr, 8);
      case 'binary':
        // 0b プレフィックスを除去
        const binStr = trimmed.replace(/^0b/i, '');
        return parseInt(binStr, 2);
      default:
        return parseInt(trimmed, 10);
    }
  };

  // セルクリック時にダイアログを開く
  const handleCellClick = (index: number) => {
    const address = startAddress + index;
    const value = values[index];
    setEditingAddress(address);
    setEditingOriginalValue(value);
    setEditInputFormat(displayFormat);

    if (typeof value === 'boolean') {
      setEditValue(value ? '1' : '0');
    } else {
      // 現在の表示形式で初期値を設定
      setEditValue(formatValueWithFormat(value, displayFormat));
    }
    setIsDialogOpen(true);
  };

  // ダイアログを閉じる
  const handleDialogClose = () => {
    setIsDialogOpen(false);
  };

  // 入力形式変更時に値を変換
  const handleInputFormatChange = (newFormat: DisplayFormat) => {
    if (!isBoolType && editValue) {
      try {
        const numValue = parseInputValue(editValue, editInputFormat);
        if (!isNaN(numValue)) {
          setEditValue(formatValueWithFormat(numValue, newFormat));
        }
      } catch {
        // パース失敗時は値をそのまま
      }
    }
    setEditInputFormat(newFormat);
  };

  // 保存処理
  const handleSave = async () => {
    try {
      if (isBoolType) {
        const newValue = editValue === 'true' || editValue === '1' || editValue.toLowerCase() === 'on';
        if (registerType === 'coils') {
          await SetCoil(editingAddress, newValue);
        } else {
          await SetDiscreteInput(editingAddress, newValue);
        }
      } else {
        const newValue = parseInputValue(editValue, editInputFormat);
        if (isNaN(newValue)) {
          console.error('Invalid number format');
          return;
        }
        if (registerType === 'holdingRegisters') {
          await SetHoldingRegister(editingAddress, newValue);
        } else {
          await SetInputRegister(editingAddress, newValue);
        }
      }
      await loadRegisters();
      setIsDialogOpen(false);
    } catch (e) {
      console.error('Failed to set register:', e);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSave();
    } else if (e.key === 'Escape') {
      handleDialogClose();
    }
  };

  const getRows = () => {
    const rows: { rowStart: number; cells: { index: number; value: boolean | number }[] }[] = [];
    for (let i = 0; i < values.length; i += COLUMNS) {
      const rowCells = values.slice(i, i + COLUMNS).map((value, j) => ({
        index: i + j,
        value
      }));
      rows.push({
        rowStart: startAddress + i,
        cells: rowCells
      });
    }
    return rows;
  };

  return (
    <div className="panel">
      <h2>レジスタ</h2>

      <div className="register-controls">
        <div className="form-group">
          <label>レジスタタイプ</label>
          <select
            value={registerType}
            onChange={(e) => setRegisterType(e.target.value as RegisterType)}
          >
            {REGISTER_TYPES.map(t => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label>開始アドレス</label>
          <input
            type="number"
            min="0"
            max="65535"
            value={startAddress}
            onChange={(e) => setStartAddress(parseInt(e.target.value) || 0)}
          />
        </div>

        {!isBoolType && (
          <div className="form-group">
            <label>表示形式</label>
            <select
              value={displayFormat}
              onChange={(e) => setDisplayFormat(e.target.value as DisplayFormat)}
            >
              {DISPLAY_FORMATS.map(f => (
                <option key={f.value} value={f.value}>{f.label}</option>
              ))}
            </select>
          </div>
        )}

        <div className="form-group">
          <label>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            自動更新
          </label>
        </div>

        <button onClick={loadRegisters} className="btn-secondary">
          更新
        </button>
      </div>

      <div className="register-matrix-container">
        <table className="register-matrix">
          <thead>
            <tr>
              <th className="row-header"></th>
              {Array.from({ length: COLUMNS }, (_, i) => (
                <th key={i} className="col-header">+{i}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {getRows().map((row) => (
              <tr key={row.rowStart}>
                <td className="row-header">{row.rowStart}</td>
                {row.cells.map((cell) => (
                  <td
                    key={cell.index}
                    onClick={() => handleCellClick(cell.index)}
                    className={`matrix-cell ${typeof cell.value === 'boolean' && cell.value ? 'cell-on' : ''}`}
                  >
                    <span className="matrix-value">{formatValue(cell.value)}</span>
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="register-navigation">
        <button
          onClick={() => setStartAddress(Math.max(0, startAddress - PAGE_SIZE))}
          disabled={startAddress === 0}
          className="btn-secondary"
        >
          前へ
        </button>
        <span>{startAddress} - {startAddress + PAGE_SIZE - 1}</span>
        <button
          onClick={() => setStartAddress(startAddress + PAGE_SIZE)}
          className="btn-secondary"
        >
          次へ
        </button>
      </div>

      {/* 書き込みダイアログ */}
      {isDialogOpen && (
        <div className="dialog-overlay">
          <div className="dialog">
            <h3>レジスタ書き込み</h3>

            <div className="dialog-content">
              <div className="dialog-row">
                <label>アドレス:</label>
                <span className="dialog-value">{editingAddress}</span>
              </div>

              <div className="dialog-row">
                <label>現在の値:</label>
                <span className="dialog-value">{formatValue(editingOriginalValue)}</span>
              </div>

              {!isBoolType && (
                <div className="dialog-row">
                  <label>入力形式:</label>
                  <select
                    value={editInputFormat}
                    onChange={(e) => handleInputFormatChange(e.target.value as DisplayFormat)}
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
                  type="text"
                  value={editValue}
                  onChange={(e) => setEditValue(e.target.value)}
                  onKeyDown={handleKeyDown}
                  autoFocus
                  className="dialog-input"
                  placeholder={isBoolType ? '0, 1, ON, OFF' : ''}
                />
              </div>
            </div>

            <div className="dialog-buttons">
              <button onClick={handleDialogClose} className="btn-secondary">
                キャンセル
              </button>
              <button onClick={handleSave} className="btn-primary">
                書き込み
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
