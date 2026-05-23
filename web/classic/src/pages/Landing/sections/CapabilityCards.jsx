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
import { Image as ImageIcon, Video, ArrowUpRight } from 'lucide-react';

const CapabilityCard = ({
  tag,
  idStamp,
  icon,
  status,
  statusLabel,
  title,
  description,
  ctaLabel,
  ctaDisabled,
  onCta,
  variant,
}) => {
  const cardClass = variant === 'live' ? 'ln-cap-card ln-cap-card--live' : 'ln-cap-card ln-cap-card--standby';
  const statusClass = status === 'live' ? 'ln-status ln-status-live' : 'ln-status ln-status-standby';

  return (
    <article className={cardClass}>
      {/* 顶部一行: 类别 tag + 状态灯 */}
      <header className='relative flex items-start justify-between'>
        <div className='flex items-center gap-3'>
          <span className='inline-flex h-9 w-9 items-center justify-center rounded-[4px] border border-white/10 bg-slate-900/60 backdrop-blur'>
            {icon}
          </span>
          <span className='ln-mono text-[10px] uppercase tracking-[0.22em] text-slate-500'>
            {tag}
          </span>
        </div>
        <span className={statusClass}>
          <span className='ln-status-dot' />
          <span>{statusLabel}</span>
        </span>
      </header>

      {/* 标题 */}
      <h3 className='ln-display relative mt-10 text-3xl font-medium leading-tight text-slate-100 sm:text-4xl'>
        {title}
      </h3>

      {/* 描述 */}
      <p className='relative mt-4 max-w-md text-sm leading-relaxed text-slate-400'>
        {description}
      </p>

      {/* CTA */}
      <div className='relative mt-auto pt-10 flex items-center'>
        {ctaDisabled ? (
          <span className='ln-cta-disabled'>
            <span>{ctaLabel}</span>
          </span>
        ) : (
          <button type='button' onClick={onCta} className='ln-cta-primary group'>
            <span>{ctaLabel}</span>
            <ArrowUpRight size={14} strokeWidth={2.2} className='transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5' />
          </button>
        )}
      </div>

      {/* 左下角 ID 戳 */}
      <span className='ln-cap-card__id'>{idStamp}</span>
    </article>
  );
};

const CapabilityCards = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <section className='ln-section relative'>
      <div className='mx-auto max-w-6xl px-6 pb-28 sm:pb-36'>
        {/* 区块小标题 */}
        <div className='landing-reveal mb-10 flex items-center gap-4'>
          <span className='ln-mono text-[10px] uppercase tracking-[0.28em] text-cyan-400/80'>
            01 / CAPABILITIES
          </span>
          <span className='ln-hairline flex-1' />
        </div>

        <div className='landing-reveal grid gap-5 sm:gap-6 lg:grid-cols-2'>
          <CapabilityCard
            tag='[ T2I ] · TEXT-TO-IMAGE'
            idStamp='#001 / IMAGE'
            icon={<ImageIcon size={16} className='text-fuchsia-300' />}
            status='live'
            statusLabel='LIVE'
            title={t('文生图')}
            description={t(
              '一句 Prompt,触达多家图像生成模型,自动选路与计费。',
            )}
            ctaLabel={t('立即体验')}
            onCta={() => navigate('/console/playground')}
            variant='live'
          />
          <CapabilityCard
            tag='[ T2V ] · TEXT-TO-VIDEO'
            idStamp='#002 / VIDEO'
            icon={<Video size={16} className='text-cyan-300' />}
            status='live'
            statusLabel='LIVE'
            title={t('文生视频')}
            description={t(
              '把灵感一键变成动态画面 —— 提交 prompt,异步出片,无需挑模型。',
            )}
            ctaLabel={t('立即体验')}
            onCta={() => navigate('/console/video-playground')}
            variant='live'
          />
        </div>
      </div>
    </section>
  );
};

export default CapabilityCards;
