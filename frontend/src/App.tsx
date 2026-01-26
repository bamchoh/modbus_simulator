import { useState } from 'react';
import './App.css';
import { ServerPanel } from './components/ServerPanel';
import { RegisterPanel } from './components/RegisterPanel';
import { ScriptPanel } from './components/ScriptPanel';

type Tab = 'server' | 'registers' | 'scripts';

function App() {
  const [activeTab, setActiveTab] = useState<Tab>('server');

  return (
    <div id="App">
      <header className="app-header">
        <h1>PLC Simulator</h1>
        <nav className="tab-nav">
          <button
            className={`tab-button ${activeTab === 'server' ? 'active' : ''}`}
            onClick={() => setActiveTab('server')}
          >
            サーバー
          </button>
          <button
            className={`tab-button ${activeTab === 'registers' ? 'active' : ''}`}
            onClick={() => setActiveTab('registers')}
          >
            レジスタ
          </button>
          <button
            className={`tab-button ${activeTab === 'scripts' ? 'active' : ''}`}
            onClick={() => setActiveTab('scripts')}
          >
            スクリプト
          </button>
        </nav>
      </header>

      <main className="app-main">
        {activeTab === 'server' && <ServerPanel />}
        {activeTab === 'registers' && <RegisterPanel />}
        {activeTab === 'scripts' && <ScriptPanel />}
      </main>
    </div>
  );
}

export default App;
