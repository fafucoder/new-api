/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import React from 'react';
import { useTranslation } from 'react-i18next';
import { MousePointer2, Terminal, Sparkles } from 'lucide-react';

const STEPS = [
  {
    icon: MousePointer2,
    titleKey: '选择模型',
    descKey: '按任务类型筛选可用模型,平台自动匹配最优渠道。',
    tag: 'SELECT',
  },
  {
    icon: Terminal,
    titleKey: '编写 Prompt',
    descKey: '在 Playground 里调参试跑,或直接调用统一 API。',
    tag: 'PROMPT',
  },
  {
    icon: Sparkles,
    titleKey: '查看结果',
    descKey: '异步任务可在控制台追踪进度,命中失败自动重试。',
    tag: 'RENDER',
  },
];

const Steps = () => {
  const { t } = useTranslation();

  return (
    <section className='ln-section relative'>
      <div className='mx-auto max-w-6xl px-6 pb-32 sm:pb-44'>
        {/* 区块头 */}
        <div className='landing-reveal mb-10 flex items-center gap-4'>
          <span className='ln-mono text-[10px] uppercase tracking-[0.28em] text-cyan-400/80'>
            03 / WORKFLOW
          </span>
          <span className='ln-hairline flex-1' />
        </div>

        <div className='landing-reveal mb-14 max-w-2xl'>
          <h2 className='ln-display text-3xl font-medium tracking-tight text-slate-100 sm:text-5xl'>
            {t('三步开始')}
          </h2>
        </div>

        <div className='landing-reveal ln-steps-track relative grid gap-10 md:grid-cols-3 md:gap-8'>
          {STEPS.map((s, i) => {
            const Icon = s.icon;
            return (
              <div key={s.titleKey} className='ln-step relative flex flex-col'>
                <div className='ln-step-connector' aria-hidden />
                <div className='relative flex items-center gap-4'>
                  <span className='ln-step-index'>
                    {String(i + 1).padStart(2, '0')}
                  </span>
                  <span className='ln-mono text-[10px] uppercase tracking-[0.28em] text-slate-500'>
                    {s.tag}
                  </span>
                </div>
                <h3 className='ln-display mt-7 flex items-center gap-3 text-2xl font-medium tracking-tight text-slate-100'>
                  <Icon size={18} strokeWidth={1.8} className='text-cyan-300' />
                  {t(s.titleKey)}
                </h3>
                <p className='mt-3 text-sm leading-relaxed text-slate-400'>
                  {t(s.descKey)}
                </p>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
};

export default Steps;
