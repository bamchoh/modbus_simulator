import { useState, useEffect } from 'react';
import {
  GetServerStatus,
  GetServerConfig,
  UpdateServerConfig,
  StartServer,
  StopServer,
  GetDisabledUnitIDs,
  SetUnitIdEnabled
} from '../../wailsjs/go/main/App';
import { application } from '../../wailsjs/go/models';

const SERVER_TYPES = [
  { value: 0, label: 'Modbus TCP' },
  { value: 1, label: 'Modbus RTU' },
  { value: 2, label: 'Modbus RTU ASCII' }
];

const PARITY_OPTIONS = [
  { value: 'N', label: 'None' },
  { value: 'E', label: 'Even' },
  { value: 'O', label: 'Odd' }
];

// 表示するUnitIDの範囲（1-247）
const UNIT_ID_RANGE = Array.from({ length: 247 }, (_, i) => i + 1);

export function ServerPanel() {
  const [status, setStatus] = useState('Stopped');
  const [config, setConfig] = useState<application.ServerConfigDTO | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isDirty, setIsDirty] = useState(false);
  const [disabledUnitIds, setDisabledUnitIds] = useState<Set<number>>(new Set());

  useEffect(() => {
    loadServerInfo();
    loadDisabledUnitIds();
    const interval = setInterval(() => {
      // ステータスのみ更新（編集中は設定を上書きしない）
      GetServerStatus().then(setStatus).catch(() => {});
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  const loadServerInfo = async () => {
    try {
      const [s, c] = await Promise.all([GetServerStatus(), GetServerConfig()]);
      setStatus(s);
      setConfig(c);
      setIsDirty(false);
    } catch (e) {
      setError(String(e));
    }
  };

  const loadDisabledUnitIds = async () => {
    try {
      const ids = await GetDisabledUnitIDs();
      setDisabledUnitIds(new Set(ids));
    } catch (e) {
      console.error('Failed to load disabled unit IDs:', e);
    }
  };

  const handleUnitIdToggle = async (unitId: number, enabled: boolean) => {
    try {
      await SetUnitIdEnabled(unitId, enabled);
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
      await loadServerInfo();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleStop = async () => {
    try {
      setError(null);
      await StopServer();
      await loadServerInfo();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleConfigChange = (field: keyof application.ServerConfigDTO, value: any) => {
    if (config) {
      setConfig({ ...config, [field]: value });
      setIsDirty(true);
    }
  };

  const handleSaveConfig = async () => {
    if (config) {
      try {
        setError(null);
        await UpdateServerConfig(config);
        await loadServerInfo();
      } catch (e) {
        setError(String(e));
      }
    }
  };

  if (!config) {
    return <div className="panel">Loading...</div>;
  }

  const isRunning = status === 'Running';
  const isTCP = config.type === 0;

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
      </div>

      {error && <div className="error-message">{error}</div>}

      <div className="config-section">
        <div className="form-group">
          <label>サーバータイプ</label>
          <select
            value={config.type}
            onChange={(e) => handleConfigChange('type', parseInt(e.target.value))}
            disabled={isRunning}
          >
            {SERVER_TYPES.map(t => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </select>
        </div>

        {isTCP ? (
          <>
            <div className="form-group">
              <label>アドレス</label>
              <input
                type="text"
                value={config.tcpAddress}
                onChange={(e) => handleConfigChange('tcpAddress', e.target.value)}
                disabled={isRunning}
              />
            </div>
            <div className="form-group">
              <label>ポート</label>
              <input
                type="number"
                min="1"
                max="65535"
                value={config.tcpPort}
                onChange={(e) => handleConfigChange('tcpPort', parseInt(e.target.value))}
                disabled={isRunning}
              />
            </div>
          </>
        ) : (
          <>
            <div className="form-group">
              <label>シリアルポート</label>
              <input
                type="text"
                value={config.serialPort}
                onChange={(e) => handleConfigChange('serialPort', e.target.value)}
                disabled={isRunning}
              />
            </div>
            <div className="form-group">
              <label>ボーレート</label>
              <select
                value={config.baudRate}
                onChange={(e) => handleConfigChange('baudRate', parseInt(e.target.value))}
                disabled={isRunning}
              >
                {[9600, 19200, 38400, 57600, 115200].map(r => (
                  <option key={r} value={r}>{r}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label>データビット</label>
              <select
                value={config.dataBits}
                onChange={(e) => handleConfigChange('dataBits', parseInt(e.target.value))}
                disabled={isRunning}
              >
                {[7, 8].map(b => (
                  <option key={b} value={b}>{b}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label>ストップビット</label>
              <select
                value={config.stopBits}
                onChange={(e) => handleConfigChange('stopBits', parseInt(e.target.value))}
                disabled={isRunning}
              >
                {[1, 2].map(b => (
                  <option key={b} value={b}>{b}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label>パリティ</label>
              <select
                value={config.parity}
                onChange={(e) => handleConfigChange('parity', e.target.value)}
                disabled={isRunning}
              >
                {PARITY_OPTIONS.map(p => (
                  <option key={p.value} value={p.value}>{p.label}</option>
                ))}
              </select>
            </div>
          </>
        )}

        <button onClick={handleSaveConfig} disabled={isRunning} className={isDirty ? 'btn-primary' : 'btn-secondary'}>
          {isDirty ? '設定を保存 *' : '設定を保存'}
        </button>
      </div>

      <div className="config-section">
        <h3>UnitID 応答設定</h3>
        <p className="help-text">オフにしたUnitIDには応答しません（デフォルト: 全て応答）</p>
        <div className="unit-id-grid">
          {UNIT_ID_RANGE.map(unitId => (
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
    </div>
  );
}
