import { useQuery } from '@tanstack/react-query'
import { Activity, Bot, Layers3, ShieldCheck, Sparkles } from 'lucide-react'

import { api } from '@/lib/api'

interface ShowcaseConfig {
  version: number
  template: string
  title: string
  subtitle: string
  announcement: string
  metrics: Array<{ label: string; value: string; trend?: string }>
  highlights: string[]
  updated_at: string
}

const themeClasses: Record<string, string> = {
  'cyber-blue': 'from-slate-950 via-blue-950 to-cyan-950 text-cyan-50',
  'executive-dark': 'from-zinc-950 via-slate-900 to-amber-950 text-amber-50',
  'operations-green': 'from-slate-950 via-emerald-950 to-teal-950 text-emerald-50',
  'aurora-purple': 'from-slate-950 via-violet-950 to-fuchsia-950 text-violet-50',
}

async function getShowcase(): Promise<ShowcaseConfig> {
  const response = await api.get('/api/skill-showcase/')
  return response.data.data.config
}

export function SkillShowcase() {
  const query = useQuery({
    queryKey: ['skill-showcase'],
    queryFn: getShowcase,
    refetchInterval: 1000,
  })
  const config = query.data
  if (!config) {
    return <div className='flex size-full items-center justify-center bg-slate-950 text-cyan-100'>正在加载展示屏...</div>
  }

  return (
    <div className={`relative min-h-full overflow-hidden bg-gradient-to-br ${themeClasses[config.template] ?? themeClasses['cyber-blue']}`}>
      <div className='absolute -left-40 -top-40 size-96 rounded-full bg-cyan-400/20 blur-3xl' />
      <div className='absolute -bottom-48 -right-24 size-[32rem] rounded-full bg-violet-500/20 blur-3xl' />
      <div className='relative mx-auto flex min-h-full max-w-7xl flex-col px-6 py-8 lg:px-12'>
        <header className='flex items-center justify-between border-b border-white/10 pb-6'>
          <div className='flex items-center gap-3'><div className='rounded-2xl bg-white/10 p-3'><Sparkles className='size-7' /></div><div><p className='text-xl font-semibold tracking-widest'>TIANYAN</p><p className='text-xs text-white/50'>ENTERPRISE AI OPERATIONS</p></div></div>
          <div className='flex items-center gap-2 rounded-full border border-emerald-300/30 bg-emerald-400/10 px-4 py-2 text-sm text-emerald-200'><span className='size-2 animate-pulse rounded-full bg-emerald-300' />实时运行</div>
        </header>

        <main className='flex flex-1 flex-col justify-center py-12'>
          <div className='mb-10 max-w-4xl'><p className='mb-4 inline-flex rounded-full border border-white/15 bg-white/5 px-4 py-2 text-sm text-white/70'>{config.announcement}</p><h1 className='text-4xl font-black tracking-tight md:text-6xl lg:text-7xl'>{config.title}</h1><p className='mt-5 text-xl text-white/60 md:text-2xl'>{config.subtitle}</p></div>
          <div className='grid gap-5 md:grid-cols-3'>{config.metrics.map((metric) => <div key={metric.label} className='group rounded-3xl border border-white/10 bg-white/[0.06] p-6 backdrop-blur-xl transition hover:-translate-y-1 hover:bg-white/10'><Activity className='mb-7 size-6 text-white/40' /><p className='text-sm text-white/50'>{metric.label}</p><p className='mt-2 text-4xl font-bold'>{metric.value}</p><p className='mt-3 text-sm text-emerald-300'>{metric.trend}</p></div>)}</div>
          <div className='mt-6 grid gap-5 md:grid-cols-3'>{config.highlights.map((item, index) => { const Icon = [Bot, ShieldCheck, Layers3][index % 3]; return <div key={item} className='flex items-center gap-4 rounded-2xl border border-white/10 bg-black/10 p-5'><Icon className='size-6 text-white/50' /><span className='font-medium'>{item}</span></div> })}</div>
        </main>
        <footer className='flex justify-between border-t border-white/10 pt-5 text-xs text-white/40'><span>由 TIANYAN Skill 受控驱动</span><span>配置版本 {config.version} · {new Date(config.updated_at).toLocaleString()}</span></footer>
      </div>
    </div>
  )
}
