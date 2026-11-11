import { describe, expect, it } from 'vitest'
import { centsFromInput, formatMoney } from '@mall/utils'

describe('money helpers', () => {
  it('formats integer cents', () => {
    expect(formatMoney(1234)).toBe('¥12.34')
  })

  it('parses yuan input without floats', () => {
    expect(centsFromInput('12.34')).toBe(1234)
    expect(centsFromInput('0.01')).toBe(1)
    expect(centsFromInput('bad')).toBe(0)
  })
})
