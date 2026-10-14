export async function readEvents(response: Response, onEvent: (event: string, value: any) => void) {
  if (!response.body || !response.headers.get('content-type')?.startsWith('text/event-stream')) throw Error('服务未返回事件流。');
  const reader = response.body.getReader(), decoder = new TextDecoder();
  let buffer = '', event = 'message', data: string[] = [], terminal = false;
  const line = (value: string) => {
    if (value === '') {
      if (data.length) {
        const body = JSON.parse(data.join('\n'));
        onEvent(event, body);
        if (event === 'error') { terminal = true; throw Error(body.message || '生成失败。'); }
        if (event === 'done') terminal = true;
      }
      event = 'message'; data = []; return;
    }
    if (value.startsWith('event:')) event = value.slice(6).trim();
    if (value.startsWith('data:')) data.push(value.slice(5).replace(/^ /, ''));
  };
  try {
    while (true) {
      const part = await reader.read();
      buffer += decoder.decode(part.value, { stream: !part.done });
      if (buffer.length > 1024 * 1024) throw Error('事件流过长。');
      let end: number;
      while ((end = buffer.indexOf('\n')) >= 0) { line(buffer.slice(0, end).replace(/\r$/, '')); buffer = buffer.slice(end + 1); }
      if (part.done) { if (buffer) line(buffer.replace(/\r$/, '')); line(''); break; }
    }
    if (!terminal) throw Error('连接已中断，请刷新会话确认状态。');
  } finally { await reader.cancel().catch(() => {}); reader.releaseLock(); }
}
