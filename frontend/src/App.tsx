import { useState } from "react";
import "./App.css";
import { ServerPanel } from "./components/ServerPanel";
import { VariableView } from "./components/VariableView";
import { RegisterPanel, RegisterTab } from "./components/RegisterPanel";
import { ScriptPanel } from "./components/ScriptPanel";
import { CommunicationIndicator } from "./components/CommunicationIndicator";

const APP_VERSION = "v0.0.13";

type Tab = "server" | "variables" | "registers" | "scripts";

function App() {
  const [activeTab, setActiveTab] = useState<Tab>("server");
  const [registerSubTab, setRegisterSubTab] = useState<RegisterTab>("list");

  return (
    <div id="App">
      <header className="app-header">
        <h1>PLC Simulator({APP_VERSION})</h1>
        <CommunicationIndicator />
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
