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
import { useNavigate } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { useLandingData } from '../useLandingData';

const PILL = {
  image: 'ln-pill-image',
  video: 'ln-pill-video',
  text: 'ln-pill-text',
};

const idOf = (name) => {
  const trimmed = (name || '').trim();
  if (!trimmed) return '??';
  const first = trimmed[0];
  return /[a-zA-Z]/.test(first) ? first.toUpperCase() : first;
};

const SkeletonCard = () => (
  <div className='ln-model-card animate-pulse'>
    <div className='h-9 w-9 rounded bg-slate-700/40' />
    <div className='mt-5 h-4 w-3/4 rounded bg-slate-700/40' />
    <div className='mt-3 h-3 w-1/3 rounded bg-slate-700/40' />
  </div>
);

const ModelGrid = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { models, totalCount, isLoading } = useLandingData();

  return (
    <section className='ln-section relative'>
      <div className='mx-auto max-w-6xl px-6 py-24 sm:py-32'>
        {/* 区块头 */}
        <div className='landing-reveal mb-10 flex items-center gap-4'>
          <span className='ln-mono text-[10px] uppercase tracking-[0.28em] text-cyan-400/80'>
            02 / CATALOG
          </span>
          <span className='ln-hairline flex-1' />
        </div>

        <div className='landing-reveal flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between'>
          <div>
            <h2 className='ln-display text-3xl font-medium tracking-tight text-slate-100 sm:text-5xl'>
              {t('已接入模型')}
            </h2>
            <p className='mt-3 max-w-xl text-sm leading-relaxed text-slate-400 sm:text-base'>
              {t('接入超过 {{n}} 家模型,覆盖图像、视频、对话等场景。', {
                n: totalCount,
              })}
            </p>
          </div>
          <button
            type='button'
            onClick={() => navigate('/pricing')}
            className='ln-link inline-flex items-center gap-1.5 self-start sm:self-end'
          >
            <span>{t('查看全部')}</span>
            <ArrowRight size={12} strokeWidth={2.3} />
          </button>
        </div>

        <div className='landing-reveal mt-10 grid grid-cols-2 gap-3 sm:gap-4 md:grid-cols-3 lg:grid-cols-4'>
          {isLoading
            ? Array.from({ length: 8 }).map((_, i) => <SkeletonCard key={i} />)
            : models.map((m, i) => (
                <article key={m.name} className='ln-model-card group'>
                  <div className='flex items-start justify-between'>
                    <span className={`ln-model-card__id ${PILL[m.category] || PILL.text}`} aria-hidden>
                      {idOf(m.name)}
                    </span>
                    <span className='ln-mono text-[9px] uppercase tracking-[0.22em] text-slate-600'>
                      #{String(i + 1).padStart(3, '0')}
                    </span>
                  </div>
                  <h4 className='mt-5 truncate text-sm font-medium text-slate-100' title={m.name}>
                    {m.name}
                  </h4>
                  <span className='mt-2 ln-mono inline-flex items-center text-[10px] uppercase tracking-[0.22em] text-slate-500'>
                    {t(
                      m.category === 'image'
                        ? '图像'
                        : m.category === 'video'
                          ? '视频'
                          : '文本',
                    )}
                  </span>
                </article>
              ))}
        </div>
      </div>
    </section>
  );
};

export default ModelGrid;
