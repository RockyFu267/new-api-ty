export interface ShowcaseDirective {
  action: 'update_showcase'
  template?: string
  changes?: Record<string, unknown>
}

export function extractShowcaseDirective(
  content: string
): ShowcaseDirective | null {
  const candidates = [
    content.match(/```tianyan-skill\s*([\s\S]*?)```/i)?.[1],
    content.match(/<tianyan-skill>\s*([\s\S]*?)<\/tianyan-skill>/i)?.[1],
    content.match(/```json\s*([\s\S]*?"action"\s*:\s*"update_showcase"[\s\S]*?)```/i)?.[1],
  ]
  for (const candidate of candidates) {
    if (!candidate) continue
    try {
      const directive = JSON.parse(candidate.trim()) as ShowcaseDirective
      if (directive.action === 'update_showcase') return directive
    } catch {
      // Continue checking other supported envelopes.
    }
  }
  return null
}

export function inferShowcaseDirective(
  userContent: string,
  assistantContent: string
): ShowcaseDirective | null {
  const intent = `${userContent}\n${assistantContent}`.toLowerCase()
  const templates: Array<[string, string[]]> = [
    ['executive-dark', ['管理驾驶舱', '高管', '商务黑金', 'executive']],
    ['operations-green', ['绿色运维', '绿色', 'operations-green']],
    ['aurora-purple', ['ai 创新', 'ai创新', '紫色', '极光', 'aurora']],
    ['cyber-blue', ['科技蓝', '蓝色', 'cyber-blue']],
  ]
  const template = templates.find(([, keywords]) =>
    keywords.some((keyword) => intent.includes(keyword))
  )?.[0]
  return template
    ? { action: 'update_showcase', template, changes: {} }
    : null
}
