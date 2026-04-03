import { useEffect, useRef } from 'react';

type EventHandler = (event: { type: string; data: any }) => void;

/**
 * 订阅 Platform SSE 事件流。
 * 收到事件时调用 onEvent 回调。
 * 组件卸载时自动断开。
 */
export function useEvents(onEvent: EventHandler) {
  const handlerRef = useRef(onEvent);
  handlerRef.current = onEvent;

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (!token) return;

    const url = `/api/v1/events?token=${encodeURIComponent(token)}`;
    const source = new EventSource(url);

    const handleMessage = (type: string) => (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data);
        handlerRef.current({ type, data });
      } catch {}
    };

    source.addEventListener('agent_online', handleMessage('agent_online'));
    source.addEventListener('agent_offline', handleMessage('agent_offline'));
    source.addEventListener('invoke_completed', handleMessage('invoke_completed'));
    source.addEventListener('invoke_status', handleMessage('invoke_status'));

    source.onerror = () => {
      // SSE 会自动重连，不需要手动处理
    };

    return () => source.close();
  }, []);
}
