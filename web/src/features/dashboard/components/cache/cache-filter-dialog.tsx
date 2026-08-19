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
import { Filter, RotateCcw, Calendar, Search } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { searchChannels } from '@/features/channels/api'
import { searchModels } from '@/features/models/api'
import {
  TIME_GRANULARITY_OPTIONS,
  TIME_RANGE_PRESETS,
} from '@/features/dashboard/constants'
import type { CacheFilters } from '@/features/dashboard/types'
import { getRollingDateRange, type TimeGranularity } from '@/lib/time'
import { cn } from '@/lib/utils'

interface CacheFilterDialogProps {
  currentFilters: CacheFilters
  onFilterChange: (filters: CacheFilters) => void
  onReset: () => void
}

function granularityForRangeDays(days: number): TimeGranularity {
  if (days <= 1) return 'hour'
  if (days >= 29) return 'week'
  return 'day'
}

function detectQuickRangeDays(
  filters: CacheFilters | undefined
): number | null {
  const start = filters?.start_timestamp
  const end = filters?.end_timestamp
  if (!start || !end) return null
  const days = Math.round((end.getTime() - start.getTime()) / 86_400_000)
  return TIME_RANGE_PRESETS.some((preset) => preset.days === days) ? days : null
}

const SectionDivider = ({ label }: { label: string }) => (
  <div className='relative'>
    <div className='absolute inset-0 flex items-center'>
      <span className='w-full border-t' />
    </div>
    <div className='relative flex justify-center text-xs uppercase'>
      <span className='bg-background text-muted-foreground px-2'>{label}</span>
    </div>
  </div>
)

export function CacheFilterDialog(props: CacheFilterDialogProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [filters, setFilters] = useState<CacheFilters>(
    () => props.currentFilters
  )
  const [selectedRange, setSelectedRange] = useState<number | null>(() =>
    detectQuickRangeDays(props.currentFilters)
  )
  const [channelOptions, setChannelOptions] = useState<
    Array<{ id: number; name: string }>
  >([])
  const [modelOptions, setModelOptions] = useState<string[]>([])

  useEffect(() => {
    if (!open) return
    searchChannels({ keyword: '', page: 1, per_page: 100 })
      .then((res) => {
        if (res.success && res.data?.items) {
          setChannelOptions(
            res.data.items.map((ch) => ({ id: ch.id, name: ch.name }))
          )
        }
      })
      .catch(() => {})
    searchModels({ keyword: '', p: 1, page_size: 200 })
      .then((res) => {
        if (res.success && res.data?.items) {
          setModelOptions(res.data.items.map((m) => m.model_name))
        }
      })
      .catch(() => {})
  }, [open])

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) {
      setFilters(props.currentFilters)
      setSelectedRange(detectQuickRangeDays(props.currentFilters))
    }
    setOpen(nextOpen)
  }

  const handleApply = () => {
    props.onFilterChange(filters)
    setOpen(false)
  }

  const handleReset = () => {
    props.onReset()
    setOpen(false)
  }

  const handleChange = (
    field: keyof CacheFilters,
    value: Date | string | number | undefined
  ) => {
    setFilters((prev) => ({ ...prev, [field]: value }))
    if (field === 'start_timestamp' || field === 'end_timestamp')
      setSelectedRange(null)
  }

  const handleQuickRange = (days: number) => {
    const { start, end } = getRollingDateRange(days)
    setFilters((prev) => ({
      ...prev,
      start_timestamp: start,
      end_timestamp: end,
      time_granularity: granularityForRangeDays(days),
    }))
    setSelectedRange(days)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      trigger={
        <Button variant='outline' size='sm'>
          <Filter className='mr-2 h-4 w-4' />
          {t('Filter')}
        </Button>
      }
      title={t('Cache Hit Rate Filters')}
      description={t(
        'Filter the cache hit rate view by time range, model, and channel.'
      )}
      contentClassName='max-sm:h-dvh max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-lg'
      contentHeight='min(48vh, 460px)'
      footerClassName='grid grid-cols-2 gap-2 sm:flex'
      footer={
        <>
          <Button onClick={handleReset} variant='outline' type='button'>
            <RotateCcw className='mr-2 h-4 w-4' />
            {t('Reset')}
          </Button>
          <Button onClick={handleApply} type='submit'>
            <Search className='mr-2 h-4 w-4' />
            {t('Apply Filters')}
          </Button>
        </>
      }
    >
      <ScrollArea className='h-full pr-3 sm:pr-4'>
        <div className='grid gap-2.5 py-2'>
          {/* Quick time range */}
          <div className='grid gap-2'>
            <Label className='flex items-center gap-2'>
              <Calendar className='h-4 w-4' />
              {t('Quick Range')}
            </Label>
            <div className='grid grid-cols-2 gap-2 sm:flex'>
              {TIME_RANGE_PRESETS.map((range) => (
                <Button
                  key={range.days}
                  type='button'
                  size='sm'
                  variant={selectedRange === range.days ? 'default' : 'outline'}
                  onClick={() => handleQuickRange(range.days)}
                  className={cn(
                    'flex-1',
                    selectedRange === range.days &&
                      'ring-ring ring-2 ring-offset-2'
                  )}
                >
                  {t(range.label)}
                </Button>
              ))}
            </div>
          </div>

          <SectionDivider label={t('Custom Time Range')} />

          {/* Custom time range */}
          <div className='grid gap-2.5'>
            <div className='grid gap-2'>
              <Label htmlFor='cache_start_timestamp'>{t('Start Time')}</Label>
              <DateTimePicker
                value={filters.start_timestamp}
                onChange={(date) =>
                  handleChange('start_timestamp', date || undefined)
                }
                placeholder={t('Select start time')}
              />
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='cache_end_timestamp'>{t('End Time')}</Label>
              <DateTimePicker
                value={filters.end_timestamp}
                onChange={(date) =>
                  handleChange('end_timestamp', date || undefined)
                }
                placeholder={t('Select end time')}
              />
            </div>
          </div>

          <SectionDivider label={t('Chart Settings')} />

          <div className='grid gap-2'>
            <Label htmlFor='cache_time_granularity'>
              {t('Time Granularity')}
            </Label>
            <Select
              items={[
                ...TIME_GRANULARITY_OPTIONS.map((option) => ({
                  value: option.value,
                  label: t(option.label),
                })),
              ]}
              value={filters.time_granularity}
              onValueChange={(value) =>
                handleChange('time_granularity', value as TimeGranularity)
              }
            >
              <SelectTrigger>
                <SelectValue placeholder={t('Select time granularity')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {TIME_GRANULARITY_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {t(option.label)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <SectionDivider label={t('Data Filters')} />

          <div className='grid gap-2'>
            <Label htmlFor='cache_model_name'>{t('Model Name')}</Label>
            <Combobox
              id='cache_model_name'
              options={modelOptions.map((name) => ({
                value: name,
                label: name,
              }))}
              value={filters.model_name || ''}
              onValueChange={(value) =>
                handleChange('model_name', value || '')
              }
              placeholder={t('Filter by model name')}
              allowCustomValue
              openOnFocus
            />
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='cache_channel_id'>{t('Channel')}</Label>
            <Select
              items={[
                { value: '0', label: t('All Channels') },
                ...channelOptions.map((ch) => ({
                  value: String(ch.id),
                  label: `${ch.name} (#${ch.id})`,
                })),
              ]}
              value={String(filters.channel_id || 0)}
              onValueChange={(value) =>
                handleChange(
                  'channel_id',
                  value === '0' ? undefined : Number(value)
                )
              }
            >
              <SelectTrigger>
                <SelectValue placeholder={t('All Channels')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value='0'>{t('All Channels')}</SelectItem>
                  {channelOptions.map((ch) => (
                    <SelectItem key={ch.id} value={String(ch.id)}>
                      {ch.name} (#{ch.id})
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        </div>
      </ScrollArea>
    </Dialog>
  )
}
