export interface SSEEvent {
  type: string
  delta?: string
  tool?: string
  error?: string
  draft_json?: string
  attribution_json?: string
}

// parseSSEChunk consumes one buffer of an SSE byte stream. It returns complete
// events and the unfinished tail, so callers can feed arbitrary chunk sizes.
export function parseSSEChunk(buffer: string): { events: SSEEvent[]; rest: string } {
  const blocks = buffer.split('\n\n')
  const rest = blocks.pop() ?? ''
  const events: SSEEvent[] = []
  for (const block of blocks) {
    const data = block
      .split('\n')
      .filter(line => line.startsWith('data:'))
      .map(line => line.slice(5).trim())
      .join('\n')
    if (!data) continue
    try {
      events.push(JSON.parse(data) as SSEEvent)
    } catch {
      // Ignore malformed frames; the server sends one JSON object per event.
    }
  }
  return { events, rest }
}
