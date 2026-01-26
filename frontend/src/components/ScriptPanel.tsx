import { useState, useEffect } from 'react';
import {
  GetScripts,
  GetIntervalPresets,
  CreateScript,
  UpdateScript,
  DeleteScript,
  StartScript,
  StopScript,
  RunScriptOnce
} from '../../wailsjs/go/main/App';
import { application } from '../../wailsjs/go/models';

const DEFAULT_CODE = `// PLCオブジェクトでレジスタにアクセスできます
// plc.getHoldingRegister(address) - 保持レジスタを読む
// plc.setHoldingRegister(address, value) - 保持レジスタに書く
// plc.getCoil(address) - コイルを読む
// plc.setCoil(address, value) - コイルに書く

// 例: カウンタをインクリメント
var count = plc.getHoldingRegister(0);
plc.setHoldingRegister(0, count + 1);
`;

export function ScriptPanel() {
  const [scripts, setScripts] = useState<application.ScriptDTO[]>([]);
  const [presets, setPresets] = useState<application.IntervalPresetDTO[]>([]);
  const [selectedScript, setSelectedScript] = useState<application.ScriptDTO | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [editName, setEditName] = useState('');
  const [editCode, setEditCode] = useState('');
  const [editInterval, setEditInterval] = useState(1000);
  const [testOutput, setTestOutput] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadData();
    const interval = setInterval(loadScripts, 1000);
    return () => clearInterval(interval);
  }, []);

  const loadData = async () => {
    await Promise.all([loadScripts(), loadPresets()]);
  };

  const loadScripts = async () => {
    try {
      const s = await GetScripts();
      setScripts(s || []);
    } catch (e) {
      console.error('Failed to load scripts:', e);
    }
  };

  const loadPresets = async () => {
    try {
      const p = await GetIntervalPresets();
      setPresets(p || []);
    } catch (e) {
      console.error('Failed to load presets:', e);
    }
  };

  const handleNew = () => {
    setSelectedScript(null);
    setIsEditing(true);
    setEditName('新しいスクリプト');
    setEditCode(DEFAULT_CODE);
    setEditInterval(1000);
    setError(null);
    setTestOutput(null);
  };

  const handleEdit = (script: application.ScriptDTO) => {
    setSelectedScript(script);
    setIsEditing(true);
    setEditName(script.name);
    setEditCode(script.code);
    setEditInterval(script.intervalMs);
    setError(null);
    setTestOutput(null);
  };

  const handleSave = async () => {
    try {
      setError(null);
      if (selectedScript) {
        await UpdateScript(selectedScript.id, editName, editCode, editInterval);
      } else {
        await CreateScript(editName, editCode, editInterval);
      }
      setIsEditing(false);
      setSelectedScript(null);
      await loadScripts();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleDelete = async (id: string) => {
    if (confirm('このスクリプトを削除しますか？')) {
      try {
        await DeleteScript(id);
        await loadScripts();
      } catch (e) {
        setError(String(e));
      }
    }
  };

  const handleToggle = async (script: application.ScriptDTO) => {
    try {
      if (script.isRunning) {
        await StopScript(script.id);
      } else {
        await StartScript(script.id);
      }
      await loadScripts();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleTest = async () => {
    try {
      setError(null);
      const result = await RunScriptOnce(editCode);
      setTestOutput(result !== undefined ? JSON.stringify(result, null, 2) : '(no output)');
    } catch (e) {
      setError(String(e));
      setTestOutput(null);
    }
  };

  const handleCancel = () => {
    setIsEditing(false);
    setSelectedScript(null);
    setError(null);
    setTestOutput(null);
  };

  if (isEditing) {
    return (
      <div className="panel">
        <h2>{selectedScript ? 'スクリプト編集' : '新しいスクリプト'}</h2>

        {error && <div className="error-message">{error}</div>}

        <div className="form-group">
          <label>名前</label>
          <input
            type="text"
            value={editName}
            onChange={(e) => setEditName(e.target.value)}
          />
        </div>

        <div className="form-group">
          <label>実行周期</label>
          <select
            value={editInterval}
            onChange={(e) => setEditInterval(parseInt(e.target.value))}
          >
            {presets.map(p => (
              <option key={p.ms} value={p.ms}>{p.label}</option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label>コード</label>
          <textarea
            value={editCode}
            onChange={(e) => setEditCode(e.target.value)}
            className="code-editor"
            spellCheck={false}
          />
        </div>

        {testOutput && (
          <div className="test-output">
            <label>テスト結果:</label>
            <pre>{testOutput}</pre>
          </div>
        )}

        <div className="button-group">
          <button onClick={handleTest} className="btn-secondary">
            テスト実行
          </button>
          <button onClick={handleSave} className="btn-primary">
            保存
          </button>
          <button onClick={handleCancel} className="btn-secondary">
            キャンセル
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="panel">
      <h2>スクリプト</h2>

      {error && <div className="error-message">{error}</div>}

      <div className="script-list-header">
        <button onClick={handleNew} className="btn-primary">
          新規作成
        </button>
      </div>

      <div className="script-list">
        {scripts.length === 0 ? (
          <p className="empty-message">スクリプトがありません</p>
        ) : (
          scripts.map(script => (
            <div key={script.id} className={`script-item ${script.isRunning ? 'running' : ''}`}>
              <div className="script-info">
                <span className="script-name">{script.name}</span>
                <span className="script-interval">
                  {presets.find(p => p.ms === script.intervalMs)?.label || `${script.intervalMs}ms`}
                </span>
                <span className={`script-status ${script.isRunning ? 'running' : 'stopped'}`}>
                  {script.isRunning ? '実行中' : '停止'}
                </span>
              </div>
              <div className="script-actions">
                <button
                  onClick={() => handleToggle(script)}
                  className={script.isRunning ? 'btn-danger' : 'btn-success'}
                >
                  {script.isRunning ? '停止' : '開始'}
                </button>
                <button onClick={() => handleEdit(script)} className="btn-secondary">
                  編集
                </button>
                <button
                  onClick={() => handleDelete(script.id)}
                  className="btn-danger"
                  disabled={script.isRunning}
                >
                  削除
                </button>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
