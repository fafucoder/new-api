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
export type AssetLibraryStatus = 'Processing' | 'Active' | 'Failed' | string

export type AssetChannelMapping = {
  id: number
  asset_id: number
  group_channel_id: number
  channel_id: number
  channel_name: string
  upstream_asset_id: string
  asset_url: string
  status: AssetLibraryStatus
  error_code: string
  error_message: string
  created_time: number
  updated_time: number
}

export type AssetLibraryAsset = {
  id: number
  group_id: number
  name: string
  asset_type: 'Image' | 'Video' | 'Audio' | string
  created_time: number
  updated_time: number
  mappings: AssetChannelMapping[]
}

export type GroupChannelMapping = {
  id: number
  group_id: number
  channel_id: number
  channel_name: string
  upstream_group_id: string
  status: AssetLibraryStatus
  error_message: string
  created_time: number
  updated_time: number
}

export type AssetLibraryGroup = {
  id: number
  display_name: string
  description: string
  group_type: string
  cover_url: string
  created_time: number
  updated_time: number
  assets: AssetLibraryAsset[]
  mappings: GroupChannelMapping[]
}

export type AssetLibraryChannel = {
  id: number
  name: string
}

export type AssetLibraryOperationResult = {
  channel_id: number
  channel_name: string
  success: boolean
  message?: string
}

export type AssetLibraryMutationData = {
  group: AssetLibraryGroup
  results: AssetLibraryOperationResult[]
}

export type ApiResponse<T> = {
  success: boolean
  message?: string
  data: T
}
