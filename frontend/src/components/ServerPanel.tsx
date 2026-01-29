import { useState, useEffect } from 'react';
import {
  GetServerStatus,
  StartServer,
  StopServer,
  ExportProject,
  ImportProject,
  GetSerialPorts,
  GetAvailableProtocols,
  GetActiveProtocol,
  GetActiveVariant,
  SetProtocol,
  GetProtocolSchema,
  GetCurrentConfig,
  UpdateConfig,
  GetUnitIDSettings,
  SetUnitIDEnabled
} from '../../wailsjs/go/main/App';
import { application } from '../../wailsjs/go/models';

export function ServerPanel() {
  const [status, setStatus] = useState('Stopped');
  const [error, setError] = useState<string | null>(null);
  const [isDirty, setIsDirty] = useState(false);

  // プロトコル関連
  const [protocols, setProtocols] = useState<application.ProtocolInfoDTO[]>([]);
  const [activeProtocol, setActiveProtocol] = useState<string>('modbus');
  const [activeVariant, setActiveVariant] = useState<string>('tcp');
  const [schema, setSchema] = useState<application.ProtocolSchemaDTO | null>(null);
  const [config, setConfig] = useState<application.ProtocolConfigDTO | null>(null);

  // UnitID設定
  const [unitIDSettings, setUnitIDSettings] = useState<application.UnitIDSettingsDTO | null>(null);
  const [disabledUnitIds, setDisabledUnitIds] = useState<Set<number>>(new Set());

  // シリアルポート
  const [serialPorts, setSerialPorts] = useState<string[]>([]);

  useEffect(() => {
    loadInitialData();
    const interval = setInterval(() => {
      GetServerStatus().then(setStatus).catch(() => {});
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  const loadInitialData = async () => {
    try {
      const [protos, active, variant] = await Promise.all([
        GetAvailableProtocols(),
        GetActiveProtocol(),
        GetActiveVariant()
      ]);
      setProtocols(protos);
      setActiveProtocol(active);
      setActiveVariant(variant);

      await Promise.all([
        loadSchema(active),
        loadConfig(),
        loadUnitIDSettings(),
        loadSerialPorts()
      ]);

      const s = await GetServerStatus();
      setStatus(s);
    } catch (e) {
      setError(String(e));
    }
  };

  const loadSchema = async (protocolType: string) => {
    try {
      const s = await GetProtocolSchema(protocolType);
      setSchema(s);
    } catch (e) {
      console.error('Failed to load schema:', e);
    }
  };

  const loadConfig = async () => {
    try {
      const cfg = await GetCurrentConfig();
      setConfig(cfg);
      setActiveVariant(cfg?.variant || 'tcp');
      setIsDirty(false);
    } catch (e) {
      console.error('Failed to load config:', e);
    }
  };

  const loadUnitIDSettings = async () => {
    try {
      const settings = await GetUnitIDSettings();
      setUnitIDSettings(settings);
      if (settings) {
        setDisabledUnitIds(new Set(settings.disabledIds || []));
      }
    } catch (e) {
      console.error('Failed to load unit ID settings:', e);
    }
  };

  const loadSerialPorts = async () => {
    try {
      const ports = await GetSerialPorts();
      setSerialPorts(ports);
    } catch (e) {
      console.error('Failed to load serial ports:', e);
    }
  };

  const handleProtocolChange = async (protocolType: string) => {
    try {
      setError(null);
      const proto = protocols.find(p => p.type === protocolType);
      const defaultVariant = proto?.variants[0]?.id || 'tcp';
      await SetProtocol(protocolType, defaultVariant);
      setActiveProtocol(protocolType);
      setActiveVariant(defaultVariant);
      await Promise.all([
        loadSchema(protocolType),
        loadConfig(),
        loadUnitIDSettings()
      ]);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleVariantChange = async (variantId: string) => {
    try {
      setError(null);
      await SetProtocol(activeProtocol, variantId);
      setActiveVariant(variantId);
      await loadConfig();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleSettingChange = (fieldName: string, value: any) => {
    if (config) {
      setConfig({
        ...config,
        settings: {
          ...config.settings,
          [fieldName]: value
        }
      });
      setIsDirty(true);
    }
  };

  const handleSaveConfig = async () => {
    if (config) {
      try {
        setError(null);
        await UpdateConfig(config);
        await loadConfig();
      } catch (e) {
        setError(String(e));
      }
    }
  };

  const handleUnitIdToggle = async (unitId: number, enabled: boolean) => {
    try {
      await SetUnitIDEnabled(unitId, enabled);
      const newDisabled = new Set(disabledUnitIds);
      if (enabled) {
        newDisabled.delete(unitId);
      } else {
        newDisabled.add(unitId);
      }
      setDisabledUnitIds(newDisabled);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleStart = async () => {
    try {
      setError(null);
      await StartServer();
      const s = await GetServerStatus();
      setStatus(s);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleStop = async () => {
    try {
      setError(null);
      await StopServer();
      const s = await GetServerStatus();
      setStatus(s);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleExport = async () => {
    try {
      setError(null);
      await ExportProject();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleImport = async () => {
    try {
      setError(null);
      await ImportProject();
      await loadInitialData();
    } catch (e) {
      setError(String(e));
    }
  };

  const isRunning = status === 'Running';

  // 現在のバリアントのフィールドを取得
  const currentVariant = schema?.variants.find(v => v.id === activeVariant);
  const fields = currentVariant?.fields || [];

  // UnitID範囲を生成
  const unitIdRange = unitIDSettings
    ? Array.from({ length: unitIDSettings.max - unitIDSettings.min + 1 }, (_, i) => unitIDSettings.min + i)
    : [];

  return (
    <div className="panel">
      <h2>サーバー設定</h2>

      <div className="status-bar">
        <span className={`status-indicator ${isRunning ? 'running' : 'stopped'}`}>
          {status}
        </span>
        <button onClick={isRunning ? handleStop : handleStart} className={isRunning ? 'btn-danger' : 'btn-primary'}>
          {isRunning ? '停止' : '開始'}
        </button>
        <div className="spacer" />
        <button onClick={handleExport} className="btn-secondary">
          エクスポート
        </button>
        <button onClick={handleImport} className="btn-secondary">
          インポート
        </button>
      </div>

      {error && <div className="error-message">{error}</div>}

      <div className="config-section">
        <div className="form-group">
          <label>プロトコル</label>
          <select
            value={activeProtocol}
            onChange={(e) => handleProtocolChange(e.target.value)}
            disabled={isRunning}
          >
            {protocols.map(p => (
              <option key={p.type} value={p.type}>{p.displayName}</option>
            ))}
          </select>
        </div>

        {schema && schema.variants.length > 1 && (
          <div className="form-group">
            <label>サーバータイプ</label>
            <select
              value={activeVariant}
              onChange={(e) => handleVariantChange(e.target.value)}
              disabled={isRunning}
            >
              {schema.variants.map(v => (
                <option key={v.id} value={v.id}>{v.displayName}</option>
              ))}
            </select>
          </div>
        )}

        {/* 動的フィールドレンダリング */}
        {fields.map(field => (
          <DynamicField
            key={field.name}
            field={field}
            value={config?.settings?.[field.name]}
            onChange={(value) => handleSettingChange(field.name, value)}
            disabled={isRunning}
            serialPorts={serialPorts}
            onRefreshPorts={loadSerialPorts}
          />
        ))}

        <button onClick={handleSaveConfig} disabled={isRunning} className={isDirty ? 'btn-primary' : 'btn-secondary'}>
          {isDirty ? '設定を保存 *' : '設定を保存'}
        </button>
      </div>

      {/* UnitID設定（プロトコルがサポートする場合のみ） */}
      {schema?.capabilities.supportsUnitId && unitIDSettings && (
        <div className="config-section">
          <h3>UnitID 応答設定</h3>
          <p className="help-text">オフにしたUnitIDには応答しません（デフォルト: 全て応答）</p>
          <div className="unit-id-grid">
            {unitIdRange.map(unitId => (
              <label key={unitId} className="unit-id-toggle">
                <input
                  type="checkbox"
                  checked={!disabledUnitIds.has(unitId)}
                  onChange={(e) => handleUnitIdToggle(unitId, e.target.checked)}
                />
                <span className="unit-id-label">{unitId}</span>
              </label>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// 動的フィールドコンポーネント
interface DynamicFieldProps {
  field: application.FieldDTO;
  value: any;
  onChange: (value: any) => void;
  disabled: boolean;
  serialPorts: string[];
  onRefreshPorts: () => void;
}

function DynamicField({ field, value, onChange, disabled, serialPorts, onRefreshPorts }: DynamicFieldProps) {
  // デフォルト値を使用
  const displayValue = value ?? field.default;

  switch (field.type) {
    case 'text':
      return (
        <div className="form-group">
          <label>{field.label}</label>
          <input
            type="text"
            value={displayValue || ''}
            onChange={(e) => onChange(e.target.value)}
            disabled={disabled}
          />
        </div>
      );

    case 'number':
      return (
        <div className="form-group">
          <label>{field.label}</label>
          <input
            type="number"
            min={field.min ?? undefined}
            max={field.max ?? undefined}
            value={displayValue ?? 0}
            onChange={(e) => onChange(parseInt(e.target.value))}
            disabled={disabled}
          />
        </div>
      );

    case 'select':
      return (
        <div className="form-group">
          <label>{field.label}</label>
          <select
            value={String(displayValue ?? '')}
            onChange={(e) => {
              // 数値の場合は数値に変換
              const val = e.target.value;
              const numVal = parseInt(val);
              onChange(isNaN(numVal) ? val : numVal);
            }}
            disabled={disabled}
          >
            {field.options?.map(opt => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
        </div>
      );

    case 'serialport':
      return (
        <div className="form-group">
          <label>{field.label}</label>
          <div style={{ display: 'flex', gap: '8px' }}>
            <select
              value={displayValue || ''}
              onChange={(e) => onChange(e.target.value)}
              disabled={disabled}
              style={{ flex: 1 }}
            >
              {serialPorts.length === 0 && (
                <option value="">ポートが見つかりません</option>
              )}
              {serialPorts.map(port => (
                <option key={port} value={port}>{port}</option>
              ))}
              {displayValue && !serialPorts.includes(displayValue) && (
                <option value={displayValue}>{displayValue} (未検出)</option>
              )}
            </select>
            <button
              onClick={onRefreshPorts}
              disabled={disabled}
              className="btn-secondary"
              title="ポート一覧を更新"
              style={{ padding: '4px 8px' }}
            >
              ↻
            </button>
          </div>
        </div>
      );

    default:
      return (
        <div className="form-group">
          <label>{field.label}</label>
          <input
            type="text"
            value={displayValue || ''}
            onChange={(e) => onChange(e.target.value)}
            disabled={disabled}
          />
        </div>
      );
  }
}
