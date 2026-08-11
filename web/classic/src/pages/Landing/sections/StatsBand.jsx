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
import React, { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

const STATS = [
  { target: 40, decimals: 0, suffix: '+', labelKey: '接入模型' },
  { target: 99.9, decimals: 1, suffix: '%', labelKey: '出片成功率' },
  { target: 1, decimals: 0, suffix: 'ms+', labelKey: '调度延迟' },
];

const easeOutExpo = (t) => (t === 1 ? 1 : 1 - Math.pow(2, -10 * t));

const useCountUp = (target, decimals, duration = 1600) => {
  const [value, setValue] = useState(0);
  const ref = useRef(null);
  const startedRef = useRef(false);

  useEffect(() => {
    const el = ref.current;
    if (!el || typeof IntersectionObserver === 'undefined') {
      setValue(target);
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting && !startedRef.current) {
            startedRef.current = true;
            const startTime = performance.now();
            const tick = (now) => {
              const elapsed = now - startTime;
              const t = Math.min(elapsed / duration, 1);
              const eased = easeOutExpo(t);
              setValue(target * eased);
              if (t < 1) requestAnimationFrame(tick);
              else setValue(target);
            };
            requestAnimationFrame(tick);
            observer.disconnect();
          }
        }
      },
      { threshold: 0.4 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [target, duration]);

  return [ref, value.toFixed(decimals)];
};

const StatCell = ({ index, stat }) => {
  const { t } = useTranslation();
  const [ref, val] = useCountUp(stat.target, stat.decimals);
  return (
    <div ref={ref} className='ln-stat-cell relative flex flex-col gap-3'>
      <div className='flex items-center gap-2'>
        <span className='ln-mono text-[10px] uppercase tracking-[0.28em] text-slate-500'>
          {String(index + 1).padStart(2, '0')}
        </span>
        <span className='h-px w-6 bg-cyan-400/30' />
        <span className='ml-1 h-1.5 w-1.5 rounded-full bg-emerald-400 shadow-[0_0_8px_1px_rgba(52,211,153,0.7)] animate-pulse' />
      </div>
      <div className='flex items-baseline gap-1'>
        <span className='ln-stat-value tabular-nums'>{val}</span>
        <span className='ln-stat-suffix'>{stat.suffix}</span>
      </div>
      <span className='ln-sparkline' aria-hidden>
        <span /><span /><span /><span /><span /><span /><span /><span />
      </span>
      <span className='max-w-[14rem] text-sm leading-relaxed text-slate-400'>
        {t(stat.labelKey)}
      </span>
    </div>
  );
};

const StatsBand = () => {
  return (
    <section className='ln-section relative'>
      <div className='mx-auto max-w-6xl px-6'>
        <div className='ln-hairline' />
        <div className='landing-reveal grid grid-cols-1 gap-12 py-16 sm:py-20 md:grid-cols-3 md:gap-6'>
          {STATS.map((s, i) => (
            <StatCell key={s.labelKey} index={i} stat={s} />
          ))}
        </div>
        <div className='ln-hairline' />
      </div>
    </section>
  );
};

export default StatsBand;
