import { useState, useEffect } from "react";
import "./App.css";
import { ServerPanel } from "./components/ServerPanel";
import { VariableView } from "./components/VariableView";
import { RegisterPanel, RegisterTab } from "./components/RegisterPanel";
import { ScriptPanel } from "./components/ScriptPanel";
import { CommunicationIndicator } from "./components/CommunicationIndicator";
import { GetHTTPAPIPort, SetHTTPAPIPort } from "../wailsjs/go/main/App";

const APP_VERSION = "v0.0.30";

type Tab = "server" | "variables" | "registers" | "scripts";

function App() {
  const [activeTab, setActiveTab] = useState<Tab>("server");
  const [registerSubTab, setRegisterSubTab] = useState<RegisterTab>("list");
  const [httpAPIPort, setHttpAPIPort] = useState<number>(8765);
  const [editingPort, setEditingPort] = useState(false);
  const [portInput, setPortInput] = useState("");
  const [portError, setPortError] = useState<string | null>(null);

  useEffect(() => {
    GetHTTPAPIPort().then(setHttpAPIPort);
  }, []);

  const handleStartEdit = () => {
    setPortInput(String(httpAPIPort));
    setPortError(null);
    setEditingPort(true);
  };

  const handleSavePort = async () => {
    const port = parseInt(portInput, 10);
    if (isNaN(port) || port < 1024 || port > 65535) {
      setPortError("1024〜65535 の範囲で入力してください");
      return;
    }
    try {
      await SetHTTPAPIPort(port);
      setHttpAPIPort(port);
      setEditingPort(false);
      setPortError(null);
    } catch (e: unknown) {
      setPortError(e instanceof Error ? e.message : String(e));
    }
  };

  const handlePortKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") handleSavePort();
    if (e.key === "Escape") setEditingPort(false);
  };

  return (
    <div id="App">
      <header className="app-header">
        <h1>PLC Simulator({APP_VERSION})</h1>
        <CommunicationIndicator />
        <div className="httpapi-indicator">
          {editingPort ? (
            <div className="httpapi-edit-form">
              <span className="httpapi-label">API port:</span>
              <input
                type="number"
                value={portInput}
                onChange={(e) => setPortInput(e.target.value)}
                onKeyDown={handlePortKeyDown}
                min={1024}
                max={65535}
                className="httpapi-port-input"
                autoFocus
              />
              <button className="httpapi-save-btn" onClick={handleSavePort}>保存</button>
              <button className="httpapi-cancel-btn" onClick={() => setEditingPort(false)}>✕</button>
              {portError && <span className="httpapi-error">{portError}</span>}
            </div>
          ) : (
            <>
              <span className="httpapi-url">http://localhost:{httpAPIPort}/api</span>
              <button className="httpapi-edit-btn" onClick={handleStartEdit} title="ポートを変更">✎</button>
            </>
          )}
        </div>
        <nav className="tab-nav">
          <button
            className={`tab-button ${activeTab === "server" ? "active" : ""}`}
            onClick={() => setActiveTab("server")}
          >
            サーバー
          </button>
          <button
            className={`tab-button ${activeTab === "variables" ? "active" : ""}`}
            onClick={() => setActiveTab("variables")}
          >
            変数
          </button>
          <button
            className={`tab-button ${activeTab === "registers" ? "active" : ""}`}
            onClick={() => setActiveTab("registers")}
          >
            レジスタ
          </button>
          <button
            className={`tab-button ${activeTab === "scripts" ? "active" : ""}`}
            onClick={() => setActiveTab("scripts")}
          >
            スクリプト
          </button>
        </nav>
      </header>

      <main className="app-main">
        {activeTab === "server" && <ServerPanel />}
        {activeTab === "variables" && <VariableView />}
        {activeTab === "registers" && (
          <RegisterPanel
            activeSubTab={registerSubTab}
            onSubTabChange={setRegisterSubTab}
          />
        )}
        {activeTab === "scripts" && <ScriptPanel />}
      </main>
    </div>
  );
}

export default App;
