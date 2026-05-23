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
import { useEffect, useMemo, useState } from 'react';
import { API } from '../../helpers';

const IMAGE_HINTS = ['image', 'flux', 'sd', 'midjourney', 'dall', 'ideogram'];
const VIDEO_HINTS = ['video', 'sora', 'kling', 'seedance', 'runway', 'pika', 'hailuo'];

const FALLBACK_MODELS = [
  { name: 'Doubao Seedance 2.0', category: 'video' },
  { name: 'Sora', category: 'video' },
  { name: 'Flux Pro', category: 'image' },
  { name: 'Midjourney v6', category: 'image' },
  { name: 'Claude Opus 4', category: 'text' },
  { name: 'Gemini 2.5 Pro', category: 'text' },
];

const classify = (name) => {
  const lower = (name || '').toLowerCase();
  if (VIDEO_HINTS.some((h) => lower.includes(h))) return 'video';
  if (IMAGE_HINTS.some((h) => lower.includes(h))) return 'image';
  return 'text';
};

export function useLandingData() {
  const [status, setStatus] = useState('loading');
  const [rawModels, setRawModels] = useState([]);

  useEffect(() => {
    let cancelled = false;
    API.get('/api/user/models', { skipErrorHandler: true })
      .then((res) => {
        if (cancelled) return;
        const data = res?.data?.data;
        if (Array.isArray(data) && data.length > 0) {
          setRawModels(data);
          setStatus('ready');
        } else {
          setStatus('empty');
        }
      })
      .catch(() => {
        if (cancelled) return;
        setStatus('error');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const models = useMemo(() => {
    if (status !== 'ready') return FALLBACK_MODELS;
    const seen = new Set();
    const out = [];
    for (const name of rawModels) {
      const key = (name || '').trim();
      if (!key || seen.has(key)) continue;
      seen.add(key);
      out.push({ name: key, category: classify(key) });
      if (out.length >= 12) break;
    }
    return out.length > 0 ? out : FALLBACK_MODELS;
  }, [status, rawModels]);

  return {
    models,
    totalCount: status === 'ready' ? rawModels.length : models.length,
    isFallback: status !== 'ready',
    isLoading: status === 'loading',
  };
}
