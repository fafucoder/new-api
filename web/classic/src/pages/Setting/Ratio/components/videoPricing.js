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

const DEFAULT_RESOLUTIONS = ['480p', '720p', '1080p'];

// Billing units. 'token' prices per 1M completion tokens (legacy default);
// 'second' prices per video second read from the request body.
export const VIDEO_UNIT_TOKEN = 'token';
export const VIDEO_UNIT_SECOND = 'second';

// Reads the video duration (seconds) from common request body shapes. Falls
// back to 0 when absent so per-second expressions never error on nil.
const DURATION_EXPR =
  '(param("duration") ?? param("metadata.duration") ?? param("seconds") ?? param("metadata.seconds") ?? 0)';

function normalizedUnit(value) {
  return value === VIDEO_UNIT_SECOND ? VIDEO_UNIT_SECOND : VIDEO_UNIT_TOKEN;
}

export function createDefaultVideoPricingConfig() {
  return {
    billingUnit: VIDEO_UNIT_TOKEN,
    rows: DEFAULT_RESOLUTIONS.flatMap((resolution) => [
      { resolution, referenceVideo: false, unitPriceUSD: 0 },
      { resolution, referenceVideo: true, unitPriceUSD: 0 },
    ]),
    defaultResolution: '720p',
  };
}

function normalizedResolution(value) {
  return String(value || '')
    .trim()
    .toLowerCase();
}

export function compareVideoPricingRows(left, right) {
  const resolutionComparison = normalizedResolution(
    left.resolution,
  ).localeCompare(normalizedResolution(right.resolution), 'en', {
    numeric: true,
    sensitivity: 'base',
  });
  if (resolutionComparison !== 0) return resolutionComparison;
  return Number(left.referenceVideo) - Number(right.referenceVideo);
}

export function sortVideoPricingRows(rows) {
  return [...rows].sort(compareVideoPricingRows);
}

function numberLiteral(value) {
  const number = Number(value);
  return String(Number.isFinite(number) && number >= 0 ? number : 0);
}

function tierLabel(row, unit, isDefault = false) {
  const unitSeg = normalizedUnit(unit) === VIDEO_UNIT_SECOND ? '|s' : '';
  return `video|${encodeURIComponent(normalizedResolution(row.resolution))}|${row.referenceVideo ? 1 : 0}${unitSeg}${isDefault ? '|default' : ''}`;
}

// Builds the cost sub-expression for a row. Per-token: c * price. Per-second:
// duration * (price * 1e6) so the backend's /1e6 quota conversion yields
// price * seconds USD.
function costExpr(row, unit) {
  const price = numberLiteral(row.unitPriceUSD);
  if (normalizedUnit(unit) === VIDEO_UNIT_SECOND) {
    return `${DURATION_EXPR} * ${numberLiteral(Number(row.unitPriceUSD) * 1000000)}`;
  }
  return `c * ${price}`;
}

export function parseVideoTierLabel(label) {
  const match = /^video\|([^|]+)\|(0|1)(\|s)?(\|default)?$/.exec(label || '');
  if (!match) return null;

  try {
    const resolution = decodeURIComponent(match[1]).trim().toLowerCase();
    if (!resolution) return null;
    return {
      resolution,
      hasVideoInput: match[2] === '1',
      unit: match[3] ? VIDEO_UNIT_SECOND : VIDEO_UNIT_TOKEN,
      isDefault: Boolean(match[4]),
    };
  } catch {
    return null;
  }
}

function resolutionCondition(resolution) {
  const normalized = normalizedResolution(resolution);
  const upper = normalized.toUpperCase();
  const values = upper === normalized ? [normalized] : [normalized, upper];
  const paths = ['metadata.resolution', 'resolution', 'metadata.size', 'size'];
  return paths
    .flatMap((path) =>
      values.map(
        (value) => `param(${JSON.stringify(path)}) == ${JSON.stringify(value)}`,
      ),
    )
    .join(' || ');
}

const REFERENCE_VIDEO_CONDITION = [
  'param("has_reference_video") == true',
  'param("metadata.has_reference_video") == true',
  'has(param("metadata.content.#.type"), "video_url")',
  'has(param("content.#.type"), "video_url")',
  'param("input_reference") != nil',
  'param("video_url") != nil',
  'param("video") != nil',
  'param("metadata.input_reference") != nil',
  'param("metadata.video_url") != nil',
  'param("metadata.video") != nil',
].join(' || ');

export function generateVideoPricingExpr(config) {
  const unit = normalizedUnit(config.billingUnit);
  const configuredRows = config.rows.filter(
    (row) => normalizedResolution(row.resolution) !== '',
  );
  const configuredResolutions = new Set(
    configuredRows.map((row) => normalizedResolution(row.resolution)),
  );
  const requestedDefault = normalizedResolution(config.defaultResolution);
  const defaultResolution = configuredResolutions.has(requestedDefault)
    ? requestedDefault
    : configuredResolutions.has('720p')
      ? '720p'
      : normalizedResolution(configuredRows[0]?.resolution || '');
  const defaultRows = configuredRows.filter(
    (row) => normalizedResolution(row.resolution) === defaultResolution,
  );
  const branches = configuredRows
    .filter((row) => normalizedResolution(row.resolution) !== defaultResolution)
    .map((row) => {
      const referenceCondition = row.referenceVideo
        ? `(${REFERENCE_VIDEO_CONDITION})`
        : `!(${REFERENCE_VIDEO_CONDITION})`;
      const condition = `(${resolutionCondition(row.resolution)}) && ${referenceCondition}`;
      const tier = `tier(${JSON.stringify(tierLabel(row, unit))}, ${costExpr(row, unit)})`;
      return `${condition} ? ${tier}`;
    });

  const defaultWithReference = defaultRows.find((row) => row.referenceVideo);
  const defaultWithoutReference = defaultRows.find(
    (row) => !row.referenceVideo,
  );
  const defaultExpr = defaultWithReference
    ? `(${REFERENCE_VIDEO_CONDITION}) ? tier(${JSON.stringify(tierLabel(defaultWithReference, unit, true))}, ${costExpr(defaultWithReference, unit)}) : ${defaultWithoutReference ? `tier(${JSON.stringify(tierLabel(defaultWithoutReference, unit, true))}, ${costExpr(defaultWithoutReference, unit)})` : 'c * 0'}`
    : defaultWithoutReference
      ? `tier(${JSON.stringify(tierLabel(defaultWithoutReference, unit, true))}, ${costExpr(defaultWithoutReference, unit)})`
      : 'c * 0';
  return [...branches, defaultExpr].join(' : ');
}

function generateLegacyVideoPricingExpr(rows, fallbackPrice) {
  const branches = rows.map((row) => {
    const referenceCondition = row.referenceVideo
      ? `(${REFERENCE_VIDEO_CONDITION})`
      : `!(${REFERENCE_VIDEO_CONDITION})`;
    const condition = `(${resolutionCondition(row.resolution)}) && ${referenceCondition}`;
    const tier = `tier(${JSON.stringify(tierLabel(row, VIDEO_UNIT_TOKEN))}, c * ${numberLiteral(row.unitPriceUSD)})`;
    return `${condition} ? ${tier}`;
  });
  return [
    ...branches,
    `tier("video|fallback", c * ${numberLiteral(fallbackPrice)})`,
  ].join(' : ');
}

export function tryParseVideoPricingConfig(exprString) {
  if (!exprString) return null;
  const body = exprString.replace(/^v\d+:/, '');

  // Per-second pricing uses a duration-based cost expression; detect it first.
  if (body.includes('param("duration")')) {
    return tryParseSecondVideoPricingConfig(body);
  }

  const rowPattern =
    /tier\("video\|([^"|]+)\|(0|1)(\|default)?",\s*c\s*\*\s*([\d.eE+-]+)\)/g;
  const rows = [];
  let defaultResolution = '';
  let match;
  while ((match = rowPattern.exec(body)) !== null) {
    let resolution;
    try {
      resolution = decodeURIComponent(match[1]);
    } catch {
      return null;
    }
    rows.push({
      resolution,
      referenceVideo: match[2] === '1',
      unitPriceUSD: Number(match[4]) || 0,
    });
    if (match[3]) defaultResolution = resolution;
  }

  if (rows.length === 0) return null;

  const legacyFallbackMatch = body.match(
    /tier\("video\|fallback",\s*c\s*\*\s*([\d.eE+-]+)\)/,
  );

  if (!defaultResolution && legacyFallbackMatch) {
    defaultResolution =
      rows.find((row) => normalizedResolution(row.resolution) === '720p')
        ?.resolution || rows[0].resolution;
    const fallbackPrice = Number(legacyFallbackMatch[1]);
    if (
      generateLegacyVideoPricingExpr(rows, fallbackPrice).replace(
        /\s+/g,
        '',
      ) !== body.replace(/\s+/g, '')
    ) {
      return null;
    }
    return {
      billingUnit: VIDEO_UNIT_TOKEN,
      rows: sortVideoPricingRows(rows),
      defaultResolution,
    };
  }

  if (!defaultResolution || legacyFallbackMatch) return null;

  const config = {
    billingUnit: VIDEO_UNIT_TOKEN,
    rows,
    defaultResolution,
  };
  if (
    generateVideoPricingExpr(config).replace(/\s+/g, '') !==
    body.replace(/\s+/g, '')
  ) {
    return null;
  }
  return { ...config, rows: sortVideoPricingRows(rows) };
}

function tryParseSecondVideoPricingConfig(body) {
  // tier("video|res|0|s", <DURATION_EXPR> * NUMBER) with optional |default.
  const rowPattern =
    /tier\("video\|([^"|]+)\|(0|1)\|s(\|default)?",\s*[^,]*?\*\s*([\d.eE+-]+)\)/g;
  const rows = [];
  let defaultResolution = '';
  let match;
  while ((match = rowPattern.exec(body)) !== null) {
    let resolution;
    try {
      resolution = decodeURIComponent(match[1]);
    } catch {
      return null;
    }
    rows.push({
      resolution,
      referenceVideo: match[2] === '1',
      // Coefficient is price * 1e6; convert back to a per-second USD price.
      unitPriceUSD: (Number(match[4]) || 0) / 1000000,
    });
    if (match[3]) defaultResolution = resolution;
  }

  if (rows.length === 0 || !defaultResolution) return null;

  const config = {
    billingUnit: VIDEO_UNIT_SECOND,
    rows,
    defaultResolution,
  };
  if (
    generateVideoPricingExpr(config).replace(/\s+/g, '') !==
    body.replace(/\s+/g, '')
  ) {
    return null;
  }
  return { ...config, rows: sortVideoPricingRows(rows) };
}
