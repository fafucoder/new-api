# Channel Management Table Sorting Feature

**Date**: 2026-06-05  
**Status**: Approved  
**Frontend**: web/classic

## Overview

Add client-side sorting functionality to three columns in the channel management table: name (名称), priority (优先级), and weight (权重). Users can click column headers to sort channels by these fields.

## Requirements

### Functional Requirements
- Support sorting by name (alphabetical)
- Support sorting by priority (numeric)
- Support sorting by weight (numeric)
- Client-side sorting (sorts data already loaded in the current page)
- No default sorting on initial page load
- Standard sort interaction: click to ascend, click to descend, click to clear

### Non-Functional Requirements
- Minimal code changes
- Follow existing codebase patterns
- Maintain compatibility with CardTable responsive wrapper
- No breaking changes to existing functionality

## Technical Design

### Architecture

No architectural changes. We leverage Semi Design Table's built-in `sorter` property to add sorting capability.

### Implementation

**File Modified**: `web/classic/src/components/table/channels/ChannelsColumnDefs.jsx`

Add `sorter` property to three column definitions within the `getChannelsColumns` function (lines 339-685).

#### 1. Name Column
**Location**: Line 339-460  
**Change**: Add `sorter` property after `dataIndex`

```javascript
{
  key: COLUMN_KEYS.NAME,
  title: t('名称'),
  dataIndex: 'name',
  sorter: (a, b) => (a.name || '').localeCompare(b.name || ''),
  render: (text, record, index) => {
    // ... existing render logic unchanged
  },
}
```

**Rationale**: 
- `localeCompare` provides proper Unicode and Chinese character sorting
- Fallback to empty string handles null/undefined values
- Works correctly for tag aggregation rows and multi-key channels

#### 2. Priority Column
**Location**: Line 576-630  
**Change**: Add `sorter` property after `dataIndex`

```javascript
{
  key: COLUMN_KEYS.PRIORITY,
  title: t('优先级'),
  dataIndex: 'priority',
  sorter: (a, b) => a.priority - b.priority,
  render: (text, record, index) => {
    // ... existing render logic unchanged
  },
}
```

**Rationale**:
- Numeric subtraction for integer sorting
- Supports negative values (min: -999) and positive values
- Simple and efficient for numeric comparison

#### 3. Weight Column
**Location**: Line 631-685  
**Change**: Add `sorter` property after `dataIndex`

```javascript
{
  key: COLUMN_KEYS.WEIGHT,
  title: t('权重'),
  dataIndex: 'weight',
  sorter: (a, b) => a.weight - b.weight,
  render: (text, record, index) => {
    // ... existing render logic unchanged
  },
}
```

**Rationale**:
- Numeric subtraction for integer sorting
- Supports values from 0 upward
- Simple and efficient for numeric comparison

### Component Flow

```
User clicks column header
    ↓
Semi Design Table component intercepts click
    ↓
Calls sorter function on all data rows
    ↓
Table re-renders with sorted data
    ↓
Sort icon updates (↑ ascending, ↓ descending, ○ none)
```

### Desktop vs Mobile Behavior

**Desktop** (Table component):
- Click column header to sort
- Sort icons appear in column headers
- Standard table sorting UI

**Mobile** (CardTable wrapper):
- Table prop passes through to Semi Design Table
- Sorting works identically on mobile
- CardTable's card rendering respects sort order

## User Experience

### Sort Interaction Pattern
1. **First click**: Ascending sort (A→Z, 小→大)
2. **Second click**: Descending sort (Z→A, 大→小)
3. **Third click**: Clear sort, return to original order

### Visual Indicators
Semi Design Table automatically shows:
- Sort icon in column header
- Active sort direction indicator
- Hover state on sortable columns

### Performance
- Client-side sorting is instant (no network request)
- Sorts only current page data (typically 10-100 rows)
- No impact on page load time

## Edge Cases

| Case | Handling |
|------|----------|
| Null/undefined name | Treated as empty string, sorted first |
| Tag aggregation rows (with children) | Included in sort like regular rows |
| Multi-key channels | Sorted normally by display name/priority/weight |
| Empty table | No sort indicators appear (no data to sort) |
| Loading state | Sort state preserved during data refresh |

## Testing Checklist

- [x] Click name column header, verify alphabetical sort (A-Z, then Z-A)
- [x] Click priority column header, verify numeric sort (low-high, then high-low)
- [x] Click weight column header, verify numeric sort (low-high, then high-low)
- [x] Click same column 3 times, verify returns to original order
- [x] Sort with tag aggregation rows present, verify all rows included
- [x] Sort with multi-key channels, verify correct ordering
- [x] Test on mobile viewport, verify CardTable maintains sort order
- [x] Switch between pages, verify sort state clears (expected behavior)
- [x] Sort empty table, verify no errors

## Dependencies

- Semi Design Table component (already in use)
- CardTable wrapper (already in use)
- No new dependencies required

## Rollout Plan

1. Implement changes to `ChannelsColumnDefs.jsx`
2. Manual testing in development
3. Deploy to production
4. No feature flag needed (non-breaking additive change)

## Future Enhancements (Out of Scope)

- Multi-column sorting (hold shift + click)
- Default sort on page load
- Server-side sorting for large datasets
- Sort persistence across page navigation
- Custom sort order configuration
