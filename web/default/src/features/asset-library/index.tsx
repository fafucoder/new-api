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
import { useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Add01Icon,
  Delete02Icon,
  FileAudioIcon,
  FileUploadIcon,
  FileVideoIcon,
  FolderOpenIcon,
  Image02Icon,
  LinkSquare02Icon,
  PencilIcon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import {
  appendAssetLibraryFiles,
  createAssetLibraryGroup,
  deleteAssetLibraryAsset,
  deleteAssetLibraryGroup,
  getAssetLibraryChannels,
  getAssetLibraryGroups,
  refreshAssetLibraryGroup,
  updateAssetLibraryGroup,
} from './api'
import type {
  AssetLibraryAsset,
  AssetLibraryGroup,
  AssetLibraryOperationResult,
  AssetLibraryStatus,
} from './types'

const queryKey = ['asset-library', 'groups'] as const

function StatusBadge({ status }: { status: AssetLibraryStatus }) {
  const { t } = useTranslation()
  const variant =
    status === 'Failed'
      ? 'destructive'
      : status === 'Active'
        ? 'default'
        : 'secondary'
  return <Badge variant={variant}>{t(status || 'Unknown')}</Badge>
}

function AssetPreview({ asset }: { asset: AssetLibraryAsset }) {
  const url = asset.mappings.find((mapping) => mapping.asset_url)?.asset_url
  if (url && asset.asset_type === 'Image') {
    return (
      <img
        src={url}
        alt={asset.name}
        className='size-12 rounded-md border object-cover'
      />
    )
  }
  if (url && asset.asset_type === 'Video') {
    return (
      <video
        src={url}
        muted
        preload='metadata'
        className='size-12 rounded-md border object-cover'
      />
    )
  }
  const Icon =
    asset.asset_type === 'Image'
      ? Image02Icon
      : asset.asset_type === 'Audio'
        ? FileAudioIcon
        : FileVideoIcon
  return (
    <span className='bg-muted text-muted-foreground flex size-12 items-center justify-center rounded-md border'>
      <HugeiconsIcon icon={Icon} className='size-5' />
    </span>
  )
}

type UploadDialogState =
  | { mode: 'create'; group: null }
  | { mode: 'append'; group: AssetLibraryGroup }
  | null

export function AssetLibrary() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [uploadDialog, setUploadDialog] = useState<UploadDialogState>(null)
  const [files, setFiles] = useState<File[]>([])
  const [displayName, setDisplayName] = useState('')
  const [editingGroup, setEditingGroup] = useState<AssetLibraryGroup | null>(
    null
  )
  const [deleteGroup, setDeleteGroup] = useState<AssetLibraryGroup | null>(null)
  const [deleteAsset, setDeleteAsset] = useState<{
    group: AssetLibraryGroup
    asset: AssetLibraryAsset
  } | null>(null)

  const groupsQuery = useQuery({ queryKey, queryFn: getAssetLibraryGroups })
  const channelsQuery = useQuery({
    queryKey: ['asset-library', 'channels'],
    queryFn: getAssetLibraryChannels,
  })

  const refreshList = async () => {
    await queryClient.invalidateQueries({ queryKey })
  }

  const notifyOperation = (
    results: AssetLibraryOperationResult[],
    successMessage: string
  ) => {
    const failed = results.filter((result) => !result.success)
    if (failed.length === 0) {
      toast.success(successMessage)
      return
    }
    toast.warning(t('Some channels failed to synchronize'), {
      description: failed
        .map((result) => result.channel_name || `#${result.channel_id}`)
        .join(', '),
    })
  }

  const uploadMutation = useMutation({
    mutationFn: async () => {
      if (!uploadDialog) throw new Error('Upload dialog is not open')
      if (uploadDialog.mode === 'create') {
        return createAssetLibraryGroup(displayName, files)
      }
      return appendAssetLibraryFiles(uploadDialog.group.id, files)
    },
    onSuccess: async (data) => {
      notifyOperation(data.results, t('Assets uploaded'))
      setUploadDialog(null)
      setFiles([])
      setDisplayName('')
      await refreshList()
    },
  })

  const editMutation = useMutation({
    mutationFn: () =>
      updateAssetLibraryGroup(editingGroup!.id, displayName.trim()),
    onSuccess: async (data) => {
      notifyOperation(data.results, t('Asset group updated'))
      setEditingGroup(null)
      setDisplayName('')
      await refreshList()
    },
  })

  const refreshMutation = useMutation({
    mutationFn: refreshAssetLibraryGroup,
    onSuccess: async (data) => {
      notifyOperation(data.results, t('Asset status refreshed'))
      await refreshList()
    },
  })

  const deleteGroupMutation = useMutation({
    mutationFn: deleteAssetLibraryGroup,
    onSuccess: async () => {
      toast.success(t('Asset group deleted'))
      setDeleteGroup(null)
      await refreshList()
    },
  })

  const deleteAssetMutation = useMutation({
    mutationFn: ({ groupId, assetId }: { groupId: number; assetId: number }) =>
      deleteAssetLibraryAsset(groupId, assetId),
    onSuccess: async () => {
      toast.success(t('Asset deleted'))
      setDeleteAsset(null)
      await refreshList()
    },
  })

  const groups = groupsQuery.data || []
  const channelSummary = useMemo(
    () =>
      t('{{count}} mapped channels', {
        count: channelsQuery.data?.length || 0,
      }),
    [channelsQuery.data?.length, t]
  )

  const openUpload = (state: UploadDialogState) => {
    if (fileInputRef.current) fileInputRef.current.value = ''
    setFiles([])
    setDisplayName('')
    setUploadDialog(state)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Asset Library')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {channelSummary}
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <Button
          disabled={
            channelsQuery.isLoading || (channelsQuery.data?.length || 0) === 0
          }
          onClick={() => openUpload({ mode: 'create', group: null })}
        >
          <HugeiconsIcon icon={FileUploadIcon} data-icon='inline-start' />
          {t('Upload assets')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {groupsQuery.isLoading ? (
          <div className='text-muted-foreground flex min-h-52 items-center justify-center'>
            <Spinner />
          </div>
        ) : groupsQuery.isError ? (
          <ErrorState
            className='min-h-64'
            onRetry={() => void groupsQuery.refetch()}
          />
        ) : groups.length === 0 ? (
          <Empty className='min-h-64 rounded-none border-y'>
            <EmptyHeader>
              <EmptyMedia variant='icon'>
                <HugeiconsIcon icon={FolderOpenIcon} />
              </EmptyMedia>
              <EmptyTitle>{t('No assets yet')}</EmptyTitle>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className='flex flex-col gap-4'>
            {groups.map((group) => (
              <Card key={group.id} className='overflow-hidden rounded-lg'>
                <CardHeader className='flex-row items-start justify-between gap-4 border-b py-4'>
                  <div className='flex min-w-0 flex-col gap-2'>
                    <div className='flex flex-wrap items-center gap-2'>
                      <CardTitle className='truncate text-base'>
                        {group.display_name}
                      </CardTitle>
                      <Badge variant='secondary'>
                        {t('{{count}} assets', { count: group.assets.length })}
                      </Badge>
                    </div>
                    <div className='flex flex-wrap gap-2'>
                      {group.mappings.map((mapping) => (
                        <Tooltip key={mapping.id}>
                          <TooltipTrigger>
                            <span className='flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs'>
                              <span>
                                {mapping.channel_name ||
                                  `#${mapping.channel_id}`}
                              </span>
                              <StatusBadge status={mapping.status} />
                            </span>
                          </TooltipTrigger>
                          {(mapping.error_message ||
                            mapping.upstream_group_id) && (
                            <TooltipContent className='max-w-80'>
                              {mapping.error_message ||
                                mapping.upstream_group_id}
                            </TooltipContent>
                          )}
                        </Tooltip>
                      ))}
                    </div>
                  </div>
                  <div className='flex shrink-0 items-center gap-1'>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant='ghost'
                            size='icon'
                            aria-label={t('Append assets')}
                            onClick={() =>
                              openUpload({ mode: 'append', group })
                            }
                          />
                        }
                      >
                        <HugeiconsIcon icon={Add01Icon} />
                      </TooltipTrigger>
                      <TooltipContent>{t('Append assets')}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant='ghost'
                            size='icon'
                            aria-label={t('Refresh status')}
                            disabled={refreshMutation.isPending}
                            onClick={() => refreshMutation.mutate(group.id)}
                          />
                        }
                      >
                        <HugeiconsIcon icon={RefreshIcon} />
                      </TooltipTrigger>
                      <TooltipContent>{t('Refresh status')}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant='ghost'
                            size='icon'
                            aria-label={t('Rename')}
                            onClick={() => {
                              setEditingGroup(group)
                              setDisplayName(group.display_name)
                            }}
                          />
                        }
                      >
                        <HugeiconsIcon icon={PencilIcon} />
                      </TooltipTrigger>
                      <TooltipContent>{t('Rename')}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant='ghost'
                            size='icon'
                            className='text-destructive hover:text-destructive'
                            aria-label={t('Delete')}
                            onClick={() => setDeleteGroup(group)}
                          />
                        }
                      >
                        <HugeiconsIcon icon={Delete02Icon} />
                      </TooltipTrigger>
                      <TooltipContent>{t('Delete')}</TooltipContent>
                    </Tooltip>
                  </div>
                </CardHeader>
                <CardContent className='p-0'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Asset')}</TableHead>
                        <TableHead>{t('Channel mappings')}</TableHead>
                        <TableHead className='w-12' />
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {group.assets.map((asset) => (
                        <TableRow key={asset.id}>
                          <TableCell>
                            <div className='flex min-w-44 items-center gap-3'>
                              <AssetPreview asset={asset} />
                              <div className='min-w-0'>
                                <div className='max-w-72 truncate text-sm font-medium'>
                                  {asset.name}
                                </div>
                                <div className='text-muted-foreground text-xs'>
                                  {t(asset.asset_type)}
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className='grid gap-2 lg:grid-cols-2'>
                              {asset.mappings.map((mapping) => (
                                <div
                                  key={mapping.id}
                                  className='flex min-w-0 items-center justify-between gap-2 rounded-md border px-2.5 py-2'
                                >
                                  <div className='min-w-0'>
                                    <div className='truncate text-xs font-medium'>
                                      {mapping.channel_name ||
                                        `#${mapping.channel_id}`}
                                    </div>
                                    <div className='text-muted-foreground truncate font-mono text-[11px]'>
                                      {mapping.error_message ||
                                        mapping.upstream_asset_id ||
                                        '-'}
                                    </div>
                                  </div>
                                  <div className='flex shrink-0 items-center gap-1.5'>
                                    <StatusBadge status={mapping.status} />
                                    {mapping.asset_url && (
                                      <Tooltip>
                                        <TooltipTrigger
                                          render={
                                            <Button
                                              variant='ghost'
                                              size='icon-sm'
                                              aria-label={t('Open asset')}
                                              render={
                                                <a
                                                  href={mapping.asset_url}
                                                  target='_blank'
                                                  rel='noreferrer'
                                                />
                                              }
                                            />
                                          }
                                        >
                                          <HugeiconsIcon
                                            icon={LinkSquare02Icon}
                                          />
                                        </TooltipTrigger>
                                        <TooltipContent>
                                          {t('Open asset')}
                                        </TooltipContent>
                                      </Tooltip>
                                    )}
                                  </div>
                                </div>
                              ))}
                            </div>
                          </TableCell>
                          <TableCell>
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button
                                    variant='ghost'
                                    size='icon'
                                    className='text-destructive hover:text-destructive'
                                    aria-label={t('Delete asset')}
                                    onClick={() =>
                                      setDeleteAsset({ group, asset })
                                    }
                                  />
                                }
                              >
                                <HugeiconsIcon icon={Delete02Icon} />
                              </TooltipTrigger>
                              <TooltipContent>
                                {t('Delete asset')}
                              </TooltipContent>
                            </Tooltip>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </SectionPageLayout.Content>

      <Dialog
        open={uploadDialog !== null}
        onOpenChange={(open) => !open && setUploadDialog(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {uploadDialog?.mode === 'append'
                ? t('Append assets')
                : t('Upload assets')}
            </DialogTitle>
            <DialogDescription>
              {t('The files will be synchronized to all mapped channels')}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            {uploadDialog?.mode === 'create' && (
              <Field>
                <FieldLabel htmlFor='asset-group-name'>
                  {t('Asset group name')}
                </FieldLabel>
                <Input
                  id='asset-group-name'
                  value={displayName}
                  maxLength={64}
                  onChange={(event) => setDisplayName(event.target.value)}
                />
              </Field>
            )}
            <Field>
              <FieldLabel>{t('Asset')}</FieldLabel>
              <input
                ref={fileInputRef}
                type='file'
                multiple
                accept='image/*,video/*,audio/*'
                className='hidden'
                onChange={(event) =>
                  setFiles(Array.from(event.target.files || []).slice(0, 20))
                }
              />
              <Button
                type='button'
                variant='outline'
                className='w-full'
                onClick={() => fileInputRef.current?.click()}
              >
                <HugeiconsIcon icon={Add01Icon} data-icon='inline-start' />
                {files.length
                  ? t('{{count}} files selected', { count: files.length })
                  : t('Select files')}
              </Button>
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button variant='outline' onClick={() => setUploadDialog(null)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={files.length === 0 || uploadMutation.isPending}
              onClick={() => uploadMutation.mutate()}
            >
              {uploadMutation.isPending && <Spinner data-icon='inline-start' />}
              {t('Upload')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={editingGroup !== null}
        onOpenChange={(open) => !open && setEditingGroup(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Rename asset group')}</DialogTitle>
          </DialogHeader>
          <Field>
            <FieldLabel htmlFor='rename-asset-group'>
              {t('Asset group name')}
            </FieldLabel>
            <Input
              id='rename-asset-group'
              value={displayName}
              maxLength={64}
              onChange={(event) => setDisplayName(event.target.value)}
            />
          </Field>
          <DialogFooter>
            <Button variant='outline' onClick={() => setEditingGroup(null)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={!displayName.trim() || editMutation.isPending}
              onClick={() => editMutation.mutate()}
            >
              {t('Save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteGroup !== null || deleteAsset !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteGroup(null)
            setDeleteAsset(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Confirm deletion')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('This action deletes the item from every mapped channel')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={() => {
                if (deleteAsset) {
                  deleteAssetMutation.mutate({
                    groupId: deleteAsset.group.id,
                    assetId: deleteAsset.asset.id,
                  })
                } else if (deleteGroup) {
                  deleteGroupMutation.mutate(deleteGroup.id)
                }
              }}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SectionPageLayout>
  )
}
