import { useState, useEffect, useRef } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';

interface Props {
  className?: string;
}

export function CommunicationIndicator({ className = '' }: Props) {
  const [rxActive, setRxActive] = useState(false);
  const [txActive, setTxActive] = useState(false);

  const rxTimeoutRef = useRef<number | null>(null);
  const txTimeoutRef = useRef<number | null>(null);

  useEffect(() => {
    // 受信イベント
    const cancelRx = EventsOn('comm:rx', () => {
      setRxActive(true);
      if (rxTimeoutRef.current) {
        clearTimeout(rxTimeoutRef.current);
      }
      rxTimeoutRef.current = window.setTimeout(() => {
        setRxActive(false);
      }, 300);
    });

    // 送信イベント
    const cancelTx = EventsOn('comm:tx', () => {
      setTxActive(true);
      if (txTimeoutRef.current) {
        clearTimeout(txTimeoutRef.current);
      }
      txTimeoutRef.current = window.setTimeout(() => {
        setTxActive(false);
      }, 300);
    });

    return () => {
      cancelRx();
      cancelTx();
      if (rxTimeoutRef.current) clearTimeout(rxTimeoutRef.current);
      if (txTimeoutRef.current) clearTimeout(txTimeoutRef.current);
    };
  }, []);

  return (
    <div className={`communication-indicator ${className}`}>
      <div className="indicator-item">
        <span className="indicator-label">RX</span>
        <span className={`indicator-lamp ${rxActive ? 'active' : ''}`} />
      </div>
      <div className="indicator-item">
        <span className="indicator-label">TX</span>
        <span className={`indicator-lamp tx ${txActive ? 'active' : ''}`} />
      </div>
    </div>
  );
}
