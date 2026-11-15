import { describe, expect, it } from 'vitest'
import { parseSSEChunk } from './sse'

describe('parseSSEChunk', () => {
  it('parses complete frames and keeps the tail', () => {
    const input = 'data: {"type":"delta","delta":"你"}\n\ndata: {"type":"done"}\n\ndata: {"type":"delta"'
    const { events, rest } = parseSSEChunk(input)
    expect(events).toEqual([{ type: 'delta', delta: '你' }, { type: 'done' }])
    expect(rest).toBe('data: {"type":"delta"')
  })

  it('returns no events for an empty buffer', () => {
    expect(parseSSEChunk('')).toEqual({ events: [], rest: '' })
  })

  it('ignores malformed JSON instead of throwing', () => {
    const { events } = parseSSEChunk('data: not-json\n\ndata: {"type":"done"}\n\n')
    expect(events).toEqual([{ type: 'done' }])
  })
})
