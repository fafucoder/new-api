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
import { ArrowRight, Play, Activity } from 'lucide-react';
import { getSystemName } from '../../../helpers';

const Hero = () => {
  const { t } = useTranslation();
  const systemName = getSystemName();

  return (
    <section className='ln-section relative overflow-hidden'>
      {/* 四角 HUD bracket */}
      <span className='ln-hud-bracket ln-hud-bracket--tl' aria-hidden />
      <span className='ln-hud-bracket ln-hud-bracket--tr' aria-hidden />
      <span className='ln-hud-bracket ln-hud-bracket--bl' aria-hidden />
      <span className='ln-hud-bracket ln-hud-bracket--br' aria-hidden />

      {/* 双辉光球 */}
      <div className='ln-glow ln-glow-cyan' aria-hidden />
      <div className='ln-glow ln-glow-green' aria-hidden />

      {/* 扫描线 */}
      <div className='ln-scanline' aria-hidden />

      <div className='relative z-10 mx-auto flex max-w-6xl flex-col px-6 pt-32 pb-32 sm:pt-40 sm:pb-44 lg:pt-48 lg:pb-52'>
        {/* 状态徽章 */}
        <div className='landing-reveal landing-reveal-1 mb-12 flex items-center gap-4'>
          <span className='ln-status ln-status-live'>
            <span className='ln-status-dot' />
            <span>LIVE</span>
          </span>
          <span className='h-px w-12 bg-white/10' />
          <span className='ln-mono text-[10px] uppercase tracking-[0.24em] text-slate-500'>
            {systemName.toUpperCase()} // CREATIVE-ENGINE
          </span>
        </div>

        {/* 标题: 三行错位组合 */}
        <h1 className='landing-reveal landing-reveal-2 max-w-5xl text-balance'>
          <span className='ln-display-mono block text-[11px] tracking-[0.3em] text-cyan-300/90 sm:text-xs'>
            [ PROMPT&nbsp;&nbsp;━━━━▶&nbsp;&nbsp;FRAME ]
          </span>
          <span
            className='ln-display ln-display-chroma mt-6 block text-5xl font-medium leading-[1.05] text-slate-100 sm:text-6xl lg:text-7xl'
            data-text={t('一句 Prompt,')}
          >
            {t('一句 Prompt,')}
          </span>
          <span className='ln-display mt-2 block text-5xl font-light italic leading-[1.05] sm:text-6xl lg:text-7xl'>
            <span className='ln-grad'>{t('一键出片.')}</span>
          </span>
        </h1>

        {/* 副标题 — 品牌名从后端动态拉取 (跟 Header 一致) */}
        <p className='landing-reveal landing-reveal-3 mt-10 max-w-2xl text-pretty text-base leading-relaxed text-slate-400 sm:text-lg'>
          <span className='font-medium text-slate-200'>{systemName}</span>{' '}
          {t(
            '把多家文生图、文生视频模型收敛成一键提交 —— 不挑模型,不拼 API,写好 prompt 就能出片。',
          )}
        </p>

        {/* CTA — 装饰性, 不挂 onClick */}
        <div className='landing-reveal landing-reveal-4 mt-14 flex flex-wrap items-center gap-4'>
          <button type='button' className='ln-cta-primary group'>
            <span>{t('立即开始')}</span>
            <ArrowRight size={14} strokeWidth={2.2} className='transition-transform group-hover:translate-x-0.5' />
          </button>
          <button type='button' className='ln-cta-ghost group'>
            <Play size={12} strokeWidth={2.5} />
            <span>{t('查看演示')}</span>
          </button>
        </div>

        {/* 底部 telemetry ticker */}
        <div className='landing-reveal landing-reveal-5 mt-24 flex flex-wrap items-center gap-x-6 gap-y-3 border-t border-white/5 pt-6 font-mono text-[10px] uppercase tracking-[0.22em] text-slate-500'>
          <span className='flex items-center gap-2'>
            <Activity size={11} className='text-cyan-400' />
            <span className='text-cyan-400'>SYS 99.9% UP</span>
          </span>
          <span className='h-3 w-px bg-white/10' />
          <span>RT&nbsp;<span className='text-slate-300'>0.847</span>ms</span>
          <span className='h-3 w-px bg-white/10' />
          <span><span className='text-slate-300'>40+</span> MODELS LIVE</span>
          <span className='h-3 w-px bg-white/10' />
          <span>RENDER QUEUE&nbsp;<span className='text-emerald-400'>NOMINAL</span></span>
        </div>
      </div>
    </section>
  );
};

export default Hero;
