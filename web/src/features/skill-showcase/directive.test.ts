import { describe, expect, it } from 'vitest'

import { extractShowcaseDirective, inferShowcaseDirective } from './directive'

describe('extractShowcaseDirective', () => {
  it('extracts a controlled showcase instruction', () => {
    expect(extractShowcaseDirective('完成\n```tianyan-skill\n{"action":"update_showcase","template":"cyber-blue"}\n```')).toEqual({ action: 'update_showcase', template: 'cyber-blue' })
  })

  it('rejects invalid or unsupported instructions', () => {
    expect(extractShowcaseDirective('```tianyan-skill\n{"action":"run_shell"}\n```')).toBeNull()
    expect(extractShowcaseDirective('没有指令')).toBeNull()
  })

  it('falls back to a safe preset inferred from the conversation', () => {
    expect(inferShowcaseDirective('切换成绿色运维风格', '已经修改好了')).toEqual({
      action: 'update_showcase',
      template: 'operations-green',
      changes: {},
    })
  })
})
