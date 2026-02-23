import { useState, useEffect } from 'react';
import {
  GetServerInstances,
  AddServer,
  RemoveServer,
  StartServer,
  StopServer,
  ExportProject,
  ImportProject,
  GetSerialPorts,
  GetAvailableProtocols,
  GetProtocolSchema,
  GetServerConfig,
  UpdateServerConfig,
  GetUnitIDSettings,
  SetUnitIDEnabled,
} from '../../wailsjs/go/main/App';
import { application } from '../../wailsjs/go/models';

// ServerInstanceRow はコンポーネント外で定義（再レンダリング時の状態リセットを防ぐため）
interface ServerInstanceRowProps {
  instance: application.ServerInstanceDTO;
  onStart: (protocolType: string) => void;
  onStop: (protocolType: string) => void;
  onRemove: (protocolType: string) => void;
  serialPorts: string[];
  onRefreshPorts: () => void;
}

function ServerInstanceRow({
  instance,
  onStart,
  onStop,
  onRemove,
  serialPorts,
  onRefreshPorts,
}: ServerInstanceRowProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [schema, setSchema] = useState<application.ProtocolSchemaDTO | null>(null);
  const [config, setConfig] = useState<application.ServerConfigDTO | null>(null);
  const [isDirty, setIsDirty] = useState(false);
  const [rowError, setRowError] = useState<string | null>(null);
  const [unitIDSettings, setUnitIDSettings] = useState<application.UnitIDSettingsDTO | null>(null);
  const [disabledUnitIds, setDisabledUnitIds] = useState<Set<number>>(new Set());

  const isRunning = instance.status === 'Running';

  const loadSettings = async () => {
    try {
      const [s, cfg, unitSettings] = await Promise.all([
        GetProtocolSchema(instance.protocolType),
        GetServerConfig(instance.protocolType),
        GetUnitIDSettings(instance.protocolType),
      ]);
      setSchema(s);
      setConfig(cfg);
      if (unitSettings) {
        setUnitIDSettings(unitSettings);
        setDisabledUnitIds(new Set(unitSettings.disabledIds || []));
      }
      setIsDirty(false);
    } catch (e) {
      setRowError(String(e));
    }
  };

  const handleToggleExpand = () => {
    if (!isExpanded) {
      loadSettings();
    }
    setIsExpanded(!isExpanded);
  };

  // 現在のバリアントのフィールドを取得
  const currentVariantId = config?.variant || instance.variant;
  const currentVariant = schema?.variants.find(v => v.id === currentVariantId);
  const fields = currentVariant?.fields || [];

  const handleVariantChange = (variantId: string) => {
    if (config) {
      setConfig({ ...config, variant: variantId });
      setIsDirty(true);
    }
  };

  const handleSettingChange = (fieldName: string, value: any) => {
    if (config) {
      setConfig({
        ...config,
        settings: {
          ...config.settings,
          [fieldName]: value,
        },
      });
      setIsDirty(true);
    }
  };

  const handleSaveConfig = async () => {
    if (config) {
      try {
        setRowError(null);
        await UpdateServerConfig(config);
        await loadSettings();
      } catch (e) {
        setRowError(String(e));
      }
    }
  };

  const handleUnitIdToggle = async (unitId: number, enabled: boolean) => {
    try {
      await SetUnitIDEnabled(instance.protocolType, unitId, enabled);
      const newDisabled = new Set(disabledUnitIds);
      if (enabled) {
        newDisabled.delete(unitId);
      } else {
        newDisabled.add(unitId);
      }
      setDisabledUnitIds(newDisabled);
    } catch (e) {
      setRowError(String(e));
    }
  };

  const unitIdRange = unitIDSettings
    ? Array.from(
        { length: unitIDSettings.max - unitIDSettings.min + 1 },
        (_, i) => unitIDSettings.min + i
      )
    : [];

  const statusClass =
    instance.status === 'Running'
      ? 'running'
      : instance.status === 'Error'
      ? 'error'
      : 'stopped';

  return (
    <div className="server-instance-row">
      <div className="server-instance-header">
        <div className="server-instance-info">
          <span className="server-instance-name">{instance.displayName}</span>
          <span className="server-instance-variant">{instance.variant}</span>
          <span className={`server-status-badge ${statusClass}`}>{instance.status}</span>
        </div>
        <div className="server-instance-actions">
          <button
            onClick={() =>
              isRunning ? onStop(instance.protocolType) : onStart(instance.protocolType)
            }
            className={isRunning ? 'btn-danger' : 'btn-primary'}
          >
            {isRunning ? '停止' : '開始'}
          </button>
          <button onClick={handleToggleExpand} className="btn-secondary">
            {isExpanded ? '設定 ▲' : '設定 ▼'}
          </button>
          <button
            onClick={() => onRemove(instance.protocolType)}
            className="btn-danger"
            disabled={isRunning}
          >
            削除
          </button>
        </div>
      </div>

      {isExpanded && (
        <div className="server-instance-config">
          {rowError && <div className="error-message">{rowError}</div>}

          {schema && schema.variants.length > 1 && config && (
            <div className="form-group">
              <label>サーバータイプ</label>
              <select
                value={config.variant}
                onChange={e => handleVariantChange(e.target.value)}
                disabled={isRunning}
              >
                {schema.variants.map(v => (
                  <option key={v.id} value={v.id}>
                    {v.displayName}
                  </option>
                ))}
              </select>
            </div>
          )}

          {fields.map(field => (
            <DynamicField
              key={field.name}
              field={field}
              value={config?.settings?.[field.name]}
              onChange={value => handleSettingChange(field.name, value)}
              disabled={isRunning}
              serialPorts={serialPorts}
              onRefreshPorts={onRefreshPorts}
            />
          ))}

          <button
            onClick={handleSaveConfig}
            disabled={isRunning}
            className={isDirty ? 'btn-primary' : 'btn-secondary'}
          >
            {isDirty ? '設定を保存 *' : '設定を保存'}
          </button>

          {schema?.capabilities.supportsUnitId && unitIDSettings && (
            <div className="unit-id-section">
              <h4>UnitID 応答設定</h4>
              <p className="help-text">
                オフにしたUnitIDには応答しません（デフォルト: 全て応答）
              </p>
              <div className="unit-id-grid">
                {unitIdRange.map(unitId => (
                  <label key={unitId} className="unit-id-toggle">
                    <input
                      type="checkbox"
                      checked={!disabledUnitIds.has(unitId)}
                      onChange={e => handleUnitIdToggle(unitId, e.target.checked)}
                    />
                    <span className="unit-id-label">{unitId}</span>
                  </label>
                ))}
              </div>
            </div>
          )}
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

function DynamicField({
  field,
  value,
  onChange,
  disabled,
  serialPorts,
  onRefreshPorts,
}: DynamicFieldProps) {
  const displayValue = value ?? field.default;

  switch (field.type) {
    case 'text':
      return (
        <div className="form-group">
          <label>{field.label}</label>
          <input
            type="text"
            value={displayValue || ''}
            onChange={e => onChange(e.target.value)}
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
            onChange={e => onChange(parseInt(e.target.value))}
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
            onChange={e => {
              const val = e.target.value;
              const numVal = parseInt(val);
              onChange(isNaN(numVal) ? val : numVal);
            }}
            disabled={disabled}
          >
            {field.options?.map(opt => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
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
              onChange={e => onChange(e.target.value)}
              disabled={disabled}
              style={{ flex: 1 }}
            >
              {serialPorts.length === 0 && (
                <option value="">ポートが見つかりません</option>
              )}
              {serialPorts.map(port => (
                <option key={port} value={port}>
                  {port}
                </option>
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
            onChange={e => onChange(e.target.value)}
            disabled={disabled}
          />
        </div>
      );
  }
}

export function ServerPanel() {
  const [serverInstances, setServerInstances] = useState<application.ServerInstanceDTO[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [serialPorts, setSerialPorts] = useState<string[]>([]);
  const [protocols, setProtocols] = useState<application.ProtocolInfoDTO[]>([]);

  // サーバー追加ダイアログ
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);
  const [selectedProtocol, setSelectedProtocol] = useState<string>('');
  const [selectedVariant, setSelectedVariant] = useState<string>('');
  const [addSchema, setAddSchema] = useState<application.ProtocolSchemaDTO | null>(null);

  useEffect(() => {
    loadInitialData();
    const interval = setInterval(() => {
      GetServerInstances()
        .then(instances => setServerInstances(instances || []))
        .catch(() => {});
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  const loadInitialData = async () => {
    try {
      const [instances, protos, ports] = await Promise.all([
        GetServerInstances(),
        GetAvailableProtocols(),
        GetSerialPorts(),
      ]);
      setServerInstances(instances || []);
      setProtocols(protos || []);
      setSerialPorts(ports || []);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleStart = async (protocolType: string) => {
    try {
      setError(null);
      await StartServer(protocolType);
      const instances = await GetServerInstances();
      setServerInstances(instances || []);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleStop = async (protocolType: string) => {
    try {
      setError(null);
      await StopServer(protocolType);
      const instances = await GetServerInstances();
      setServerInstances(instances || []);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleRemove = async (protocolType: string) => {
    try {
      setError(null);
      await RemoveServer(protocolType);
      const instances = await GetServerInstances();
      setServerInstances(instances || []);
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

  const handleRefreshPorts = async () => {
    try {
      const ports = await GetSerialPorts();
      setSerialPorts(ports || []);
    } catch (e) {
      console.error('Failed to load serial ports:', e);
    }
  };

  // 追加済みでないプロトコルのみ候補に
  const addedProtocolTypes = new Set(serverInstances.map(i => i.protocolType));
  const availableProtocols = protocols.filter(p => !addedProtocolTypes.has(p.type));

  const handleOpenAddDialog = async () => {
    if (availableProtocols.length === 0) return;
    const firstProto = availableProtocols[0];
    try {
      const schema = await GetProtocolSchema(firstProto.type);
      setAddSchema(schema);
      setSelectedProtocol(firstProto.type);
      setSelectedVariant(schema?.variants[0]?.id || '');
    } catch (e) {
      setError(String(e));
      return;
    }
    setIsAddDialogOpen(true);
  };

  const handleAddProtocolChange = async (protocolType: string) => {
    try {
      const schema = await GetProtocolSchema(protocolType);
      setAddSchema(schema);
      setSelectedProtocol(protocolType);
      setSelectedVariant(schema?.variants[0]?.id || '');
    } catch (e) {
      setError(String(e));
    }
  };

  const handleAddServer = async () => {
    try {
      setError(null);
      await AddServer(selectedProtocol, selectedVariant);
      const instances = await GetServerInstances();
      setServerInstances(instances || []);
      setIsAddDialogOpen(false);
    } catch (e) {
      setError(String(e));
    }
  };

  // ESCキーで追加ダイアログを閉じる
  useEffect(() => {
    if (!isAddDialogOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsAddDialogOpen(false);
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isAddDialogOpen]);

  return (
    <div className="panel">
      <div className="server-panel-header">
        <h2>サーバー</h2>
        <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <button
            onClick={handleOpenAddDialog}
            disabled={availableProtocols.length === 0}
            className="btn-primary"
          >
            + サーバーを追加
          </button>
          <button onClick={handleExport} className="btn-secondary">
            エクスポート
          </button>
          <button onClick={handleImport} className="btn-secondary">
            インポート
          </button>
        </div>
      </div>

      {error && <div className="error-message">{error}</div>}

      <div className="server-instance-list">
        {serverInstances.length === 0 ? (
          <div className="server-instance-empty">
            サーバーが登録されていません。「+ サーバーを追加」ボタンでサーバーを追加してください。
          </div>
        ) : (
          serverInstances.map(instance => (
            <ServerInstanceRow
              key={instance.protocolType}
              instance={instance}
              onStart={handleStart}
              onStop={handleStop}
              onRemove={handleRemove}
              serialPorts={serialPorts}
              onRefreshPorts={handleRefreshPorts}
            />
          ))
        )}
      </div>

      {/* サーバー追加ダイアログ */}
      {isAddDialogOpen && (
        <div className="dialog-overlay">
          <div className="dialog">
            <h3>サーバーを追加</h3>
            <div className="dialog-content">
              <div className="form-group">
                <label>プロトコル</label>
                <select
                  value={selectedProtocol}
                  onChange={e => handleAddProtocolChange(e.target.value)}
                >
                  {availableProtocols.map(p => (
                    <option key={p.type} value={p.type}>
                      {p.displayName}
                    </option>
                  ))}
                </select>
              </div>
              {addSchema && addSchema.variants.length > 1 && (
                <div className="form-group">
                  <label>バリアント</label>
                  <select
                    value={selectedVariant}
                    onChange={e => setSelectedVariant(e.target.value)}
                  >
                    {addSchema.variants.map(v => (
                      <option key={v.id} value={v.id}>
                        {v.displayName}
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </div>
            <div className="dialog-buttons">
              <button onClick={() => setIsAddDialogOpen(false)} className="btn-secondary">
                キャンセル
              </button>
              <button onClick={handleAddServer} className="btn-primary">
                追加
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
