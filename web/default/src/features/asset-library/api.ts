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
import { api } from '@/lib/api'
import type {
  ApiResponse,
  AssetLibraryChannel,
  AssetLibraryGroup,
  AssetLibraryMutationData,
  AssetLibraryOperationResult,
} from './types'

export async function getAssetLibraryChannels() {
  const response = await api.get<ApiResponse<AssetLibraryChannel[]>>(
    '/api/asset-library/channels'
  )
  return response.data.data
}

export async function getAssetLibraryGroups() {
  const response = await api.get<ApiResponse<AssetLibraryGroup[]>>(
    '/api/asset-library/groups'
  )
  return response.data.data
}

export async function createAssetLibraryGroup(
  displayName: string,
  files: File[]
) {
  const form = new FormData()
  if (displayName.trim()) form.append('display_name', displayName.trim())
  files.forEach((file) => form.append('files', file))
  const response = await api.post<ApiResponse<AssetLibraryMutationData>>(
    '/api/asset-library/groups',
    form
  )
  return response.data.data
}

export async function appendAssetLibraryFiles(groupId: number, files: File[]) {
  const form = new FormData()
  files.forEach((file) => form.append('files', file))
  const response = await api.post<ApiResponse<AssetLibraryMutationData>>(
    `/api/asset-library/groups/${groupId}/assets`,
    form
  )
  return response.data.data
}

export async function updateAssetLibraryGroup(
  groupId: number,
  displayName: string
) {
  const response = await api.patch<ApiResponse<AssetLibraryMutationData>>(
    `/api/asset-library/groups/${groupId}`,
    { display_name: displayName }
  )
  return response.data.data
}

export async function refreshAssetLibraryGroup(groupId: number) {
  const response = await api.post<ApiResponse<AssetLibraryMutationData>>(
    `/api/asset-library/groups/${groupId}/refresh`
  )
  return response.data.data
}

export async function deleteAssetLibraryGroup(groupId: number) {
  const response = await api.delete<ApiResponse<AssetLibraryOperationResult[]>>(
    `/api/asset-library/groups/${groupId}`
  )
  return response.data.data
}

export async function deleteAssetLibraryAsset(
  groupId: number,
  assetId: number
) {
  const response = await api.delete<ApiResponse<AssetLibraryOperationResult[]>>(
    `/api/asset-library/groups/${groupId}/assets/${assetId}`
  )
  return response.data.data
}
