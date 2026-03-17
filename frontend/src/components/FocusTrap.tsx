import React, { useRef, useEffect } from "react";

const FOCUSABLE_SELECTORS = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
].join(", ");

interface FocusTrapProps {
  children: React.ReactNode;
  className?: string;
  /** Enter キー押下時に呼ばれる確定ハンドラ。button / textarea フォーカス中は呼ばれない */
  onConfirm?: () => void;
}

/**
 * ダイアログ等に使うフォーカストラップ。
 * マウント時に直前のフォーカス要素を記録し、アンマウント時（ダイアログを閉じたとき）に復元する。
 * マウント中はコンテナ内でタブフォーカスをループさせる。
 * onConfirm を渡すと Enter キーで確定できる（button / textarea 除く）。
 * dialog-overlay の代わりに使用する。
 */
export function FocusTrap({ children, className, onConfirm }: FocusTrapProps) {
  const ref = useRef<HTMLDivElement>(null);

  // レンダー時点（DOM コミット・autoFocus より前）のフォーカス要素を記録。
  // useEffect 内で取得すると autoFocus 後になりダイアログ内要素を誤記録するため useRef の初期値で取得する。
  const previousFocused = useRef(document.activeElement as HTMLElement | null);

  // ダイアログ内にフォーカスを移動（autoFocus 要素がある場合はそちらに任せ、ない場合は最初の要素へ）
  useEffect(() => {
    const container = ref.current;
    if (!container) return;
    // すでにコンテナ内にフォーカスがある（autoFocus 発火済み）場合はスキップ
    if (container.contains(document.activeElement)) return;
    const first = container.querySelector<HTMLElement>(FOCUSABLE_SELECTORS);
    first?.focus();
  }, []);

  // フォーカス復元: アンマウント時（ダイアログを閉じたとき）に返す
  useEffect(() => {
    return () => {
      previousFocused.current?.focus();
    };
  }, []);

  // キーボードイベント処理
  useEffect(() => {
    // マウント時刻を記録。ダイアログを開いた Enter キーイベント（timeStamp がこれ以前）を無視するため
    const mountedAt = performance.now();

    const handleKeyDown = (e: KeyboardEvent) => {
      const container = ref.current;
      if (!container) return;

      // Tab フォーカストラップ
      if (e.key === "Tab") {
        const focusable = Array.from(
          container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTORS),
        ).filter((el) => el.offsetParent !== null);

        if (focusable.length === 0) return;

        const first = focusable[0];
        const last = focusable[focusable.length - 1];

        if (e.shiftKey) {
          if (document.activeElement === first) {
            e.preventDefault();
            last.focus();
          }
        } else {
          if (document.activeElement === last) {
            e.preventDefault();
            first.focus();
          }
        }
        return;
      }

      // Enter で確定
      if (e.key === "Enter" && onConfirm) {
        // マウント前後 100ms 以内のイベントは「ダイアログを開いた Enter」として無視
        if (e.timeStamp < mountedAt + 100) return;
        const tag = (document.activeElement as HTMLElement)?.tagName;
        // ボタン（Enter = click）・textarea（Enter = 改行）は除外
        if (tag === "BUTTON" || tag === "TEXTAREA") return;
        e.preventDefault();
        onConfirm();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onConfirm]);

  return (
    <div ref={ref} className={className ?? "dialog-overlay"}>
      {children}
    </div>
  );
}
