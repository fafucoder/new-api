/*
Copyright (C) 2023-2026 QuantumNous

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

export type VideoPricingRow = {
  resolution: string
  referenceVideo: boolean
  unitPriceUSD: number
}

export type VideoPricingConfig = {
  rows: VideoPricingRow[]
  defaultResolution: string
}

export type VideoTierLabel = {
  resolution: string
  hasVideoInput: boolean
  isDefault: boolean
}

const DEFAULT_RESOLUTIONS = ['480p', '720p', '1080p']

export function createDefaultVideoPricingConfig(): VideoPricingConfig {
  return {
    rows: DEFAULT_RESOLUTIONS.flatMap((resolution) => [
      { resolution, referenceVideo: false, unitPriceUSD: 0 },
      { resolution, referenceVideo: true, unitPriceUSD: 0 },
    ]),
    defaultResolution: '720p',
  }
}

function normalizedResolution(value: string): string {
  return value.trim().toLowerCase()
}

export function compareVideoPricingRows(
  left: Pick<VideoPricingRow, 'resolution' | 'referenceVideo'>,
  right: Pick<VideoPricingRow, 'resolution' | 'referenceVideo'>
): number {
  const resolutionComparison = normalizedResolution(
    left.resolution
  ).localeCompare(normalizedResolution(right.resolution), 'en', {
    numeric: true,
    sensitivity: 'base',
  })
  if (resolutionComparison !== 0) return resolutionComparison
  return Number(left.referenceVideo) - Number(right.referenceVideo)
}

export function sortVideoPricingRows(
  rows: VideoPricingRow[]
): VideoPricingRow[] {
  return [...rows].sort(compareVideoPricingRows)
}

function numberLiteral(value: number): string {
  return String(Number.isFinite(value) && value >= 0 ? value : 0)
}

function tierLabel(row: VideoPricingRow, isDefault = false): string {
  return `video|${encodeURIComponent(normalizedResolution(row.resolution))}|${row.referenceVideo ? 1 : 0}${isDefault ? '|default' : ''}`
}

export function parseVideoTierLabel(
  label: string | null | undefined
): VideoTierLabel | null {
  const match = /^video\|([^|]+)\|(0|1)(\|default)?$/.exec(label || '')
  if (!match) return null

  try {
    const resolution = decodeURIComponent(match[1]).trim().toLowerCase()
    if (!resolution) return null
    return {
      resolution,
      hasVideoInput: match[2] === '1',
      isDefault: Boolean(match[3]),
    }
  } catch {
    return null
  }
}

function resolutionCondition(resolution: string): string {
  const normalized = normalizedResolution(resolution)
  const upper = normalized.toUpperCase()
  const values = upper === normalized ? [normalized] : [normalized, upper]
  const paths = ['metadata.resolution', 'resolution', 'metadata.size', 'size']
  return paths
    .flatMap((path) =>
      values.map(
        (value) => `param(${JSON.stringify(path)}) == ${JSON.stringify(value)}`
      )
    )
    .join(' || ')
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
].join(' || ')

export function generateVideoPricingExpr(config: VideoPricingConfig): string {
  const configuredRows = config.rows.filter(
    (row) => normalizedResolution(row.resolution) !== ''
  )
  const configuredResolutions = new Set(
    configuredRows.map((row) => normalizedResolution(row.resolution))
  )
  const requestedDefault = normalizedResolution(config.defaultResolution)
  const defaultResolution = configuredResolutions.has(requestedDefault)
    ? requestedDefault
    : configuredResolutions.has('720p')
      ? '720p'
      : normalizedResolution(configuredRows[0]?.resolution || '')
  const defaultRows = configuredRows.filter(
    (row) => normalizedResolution(row.resolution) === defaultResolution
  )
  const branches = configuredRows
    .filter((row) => normalizedResolution(row.resolution) !== defaultResolution)
    .map((row) => {
      const referenceCondition = row.referenceVideo
        ? `(${REFERENCE_VIDEO_CONDITION})`
        : `!(${REFERENCE_VIDEO_CONDITION})`
      const condition = `(${resolutionCondition(row.resolution)}) && ${referenceCondition}`
      const tier = `tier(${JSON.stringify(tierLabel(row))}, c * ${numberLiteral(row.unitPriceUSD)})`
      return `${condition} ? ${tier}`
    })

  const defaultWithReference = defaultRows.find((row) => row.referenceVideo)
  const defaultWithoutReference = defaultRows.find((row) => !row.referenceVideo)
  const defaultExpr = defaultWithReference
    ? `(${REFERENCE_VIDEO_CONDITION}) ? tier(${JSON.stringify(tierLabel(defaultWithReference, true))}, c * ${numberLiteral(defaultWithReference.unitPriceUSD)}) : ${defaultWithoutReference ? `tier(${JSON.stringify(tierLabel(defaultWithoutReference, true))}, c * ${numberLiteral(defaultWithoutReference.unitPriceUSD)})` : 'c * 0'}`
    : defaultWithoutReference
      ? `tier(${JSON.stringify(tierLabel(defaultWithoutReference, true))}, c * ${numberLiteral(defaultWithoutReference.unitPriceUSD)})`
      : 'c * 0'
  return [...branches, defaultExpr].join(' : ')
}

function generateLegacyVideoPricingExpr(
  rows: VideoPricingRow[],
  fallbackPrice: number
): string {
  const branches = rows.map((row) => {
    const referenceCondition = row.referenceVideo
      ? `(${REFERENCE_VIDEO_CONDITION})`
      : `!(${REFERENCE_VIDEO_CONDITION})`
    const condition = `(${resolutionCondition(row.resolution)}) && ${referenceCondition}`
    const tier = `tier(${JSON.stringify(tierLabel(row))}, c * ${numberLiteral(row.unitPriceUSD)})`
    return `${condition} ? ${tier}`
  })
  return [
    ...branches,
    `tier("video|fallback", c * ${numberLiteral(fallbackPrice)})`,
  ].join(' : ')
}

export function tryParseVideoPricingConfig(
  exprString: string | null | undefined
): VideoPricingConfig | null {
  if (!exprString) return null
  const body = exprString.replace(/^v\d+:/, '')
  const rowPattern =
    /tier\("video\|([^"|]+)\|(0|1)(\|default)?",\s*c\s*\*\s*([\d.eE+-]+)\)/g
  const rows: VideoPricingRow[] = []
  let defaultResolution = ''
  let match: RegExpExecArray | null
  while ((match = rowPattern.exec(body)) !== null) {
    let resolution: string
    try {
      resolution = decodeURIComponent(match[1])
    } catch {
      return null
    }
    rows.push({
      resolution,
      referenceVideo: match[2] === '1',
      unitPriceUSD: Number(match[4]) || 0,
    })
    if (match[3]) defaultResolution = resolution
  }
  if (rows.length === 0) return null

  const legacyFallbackMatch = body.match(
    /tier\("video\|fallback",\s*c\s*\*\s*([\d.eE+-]+)\)/
  )

  if (!defaultResolution && legacyFallbackMatch) {
    defaultResolution =
      rows.find((row) => normalizedResolution(row.resolution) === '720p')
        ?.resolution || rows[0].resolution
    const fallbackPrice = Number(legacyFallbackMatch[1])
    if (
      generateLegacyVideoPricingExpr(rows, fallbackPrice).replace(
        /\s+/g,
        ''
      ) !== body.replace(/\s+/g, '')
    ) {
      return null
    }
    return { rows: sortVideoPricingRows(rows), defaultResolution }
  }

  if (!defaultResolution || legacyFallbackMatch) return null

  const config: VideoPricingConfig = {
    rows,
    defaultResolution,
  }
  if (
    generateVideoPricingExpr(config).replace(/\s+/g, '') !==
    body.replace(/\s+/g, '')
  ) {
    return null
  }
  return { ...config, rows: sortVideoPricingRows(rows) }
}
