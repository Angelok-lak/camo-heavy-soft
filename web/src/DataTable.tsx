// The one list component (D-09): TanStack Table drives sorting and
// filtering, the markup keeps the app's table classes. Screens declare
// columns, this renders them.

import { useState } from 'react'
import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table'

export default function DataTable<T>({
  data,
  columns,
  globalFilter,
  emptyText,
}: {
  data: T[]
  columns: ColumnDef<T, unknown>[]
  globalFilter?: string
  emptyText: string
}) {
  const [sorting, setSorting] = useState<SortingState>([])

  const table = useReactTable({
    data,
    columns,
    state: { sorting, globalFilter },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    globalFilterFn: 'includesString',
  })

  return (
    <table className="data">
      <thead>
        {table.getHeaderGroups().map((hg) => (
          <tr key={hg.id}>
            {hg.headers.map((h) => {
              const sortable = h.column.getCanSort()
              const dir = h.column.getIsSorted()
              return (
                <th
                  key={h.id}
                  className={sortable ? 'sortable' : ''}
                  onClick={sortable ? h.column.getToggleSortingHandler() : undefined}
                >
                  {flexRender(h.column.columnDef.header, h.getContext())}
                  {dir && <span className="arrow">{dir === 'asc' ? '▲' : '▼'}</span>}
                </th>
              )
            })}
          </tr>
        ))}
      </thead>
      <tbody>
        {table.getRowModel().rows.map((row) => (
          <tr key={row.id} className={(row.original as { _rowClass?: string })._rowClass ?? ''}>
            {row.getVisibleCells().map((cell) => (
              <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>
            ))}
          </tr>
        ))}
        {table.getRowModel().rows.length === 0 && (
          <tr>
            <td colSpan={columns.length} className="muted">
              {emptyText}
            </td>
          </tr>
        )}
      </tbody>
    </table>
  )
}
