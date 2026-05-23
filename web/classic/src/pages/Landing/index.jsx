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
import React, { useEffect, useMemo, useRef } from 'react';
import Hero from './sections/Hero';
import CapabilityCards from './sections/CapabilityCards';
import StatsBand from './sections/StatsBand';
import ModelGrid from './sections/ModelGrid';
import Steps from './sections/Steps';
import './Landing.css';

const PARTICLE_COUNT = 36;
const PARTICLE_COLORS = ['#67e8f9', '#c084fc', '#f0abfc', '#34d399'];

const Landing = () => {
  const rootRef = useRef(null);

  // 粒子层 — 用确定性"伪随机"避免 SSR mismatch / re-render 抖动
  const particles = useMemo(() => {
    return Array.from({ length: PARTICLE_COUNT }, (_, i) => ({
      left: `${(i * 37) % 100}%`,
      delay: `-${((i * 1.3) % 18).toFixed(2)}s`,
      duration: `${16 + (i % 6) * 3}s`,
      drift: `${((i % 11) - 5) * 18}px`,
      color: PARTICLE_COLORS[i % PARTICLE_COLORS.length],
      size: i % 7 === 0 ? 3 : 2,
    }));
  }, []);

  useEffect(() => {
    const root = rootRef.current;
    if (!root || typeof IntersectionObserver === 'undefined') {
      root?.querySelectorAll('.landing-reveal')?.forEach((el) => {
        el.classList.add('is-visible');
      });
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            entry.target.classList.add('is-visible');
            observer.unobserve(entry.target);
          }
        }
      },
      { threshold: 0.15, rootMargin: '0px 0px -80px 0px' },
    );
    const targets = root.querySelectorAll('.landing-reveal');
    targets.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={rootRef} className='landing-root'>
      {/* 极光 mesh 背景 */}
      <div className='ln-aurora' aria-hidden>
        <span className='ln-aurora__blob ln-aurora__blob--violet' />
        <span className='ln-aurora__blob ln-aurora__blob--magenta' />
        <span className='ln-aurora__blob ln-aurora__blob--cyan' />
        <span className='ln-aurora__blob ln-aurora__blob--emerald' />
      </div>

      {/* 粒子层 */}
      <div className='ln-particles' aria-hidden>
        {particles.map((p, i) => (
          <span
            key={i}
            className='ln-particle'
            style={{
              left: p.left,
              color: p.color,
              width: `${p.size}px`,
              height: `${p.size}px`,
              animationDelay: p.delay,
              animationDuration: p.duration,
              '--ln-tx': p.drift,
            }}
          />
        ))}
      </div>

      {/* 内容层 */}
      <div className='relative' style={{ zIndex: 5 }}>
        <Hero />
        <CapabilityCards />
        <StatsBand />
        <ModelGrid />
        <Steps />
      </div>
    </div>
  );
};

export default Landing;
