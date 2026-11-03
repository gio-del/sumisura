import type { Priority } from '@/api/types'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export const PRIORITIES: Priority[] = ['high', 'medium', 'low']

export const PRIORITY_LABELS: Record<Priority, string> = {
  high: 'High',
  medium: 'Medium',
  low: 'Low',
}

const NOT_SET = 'none'

interface Props {
  value: Priority | undefined
  disabled?: boolean
  // label names the control for assistive tech, e.g. the Job Listing's company.
  label?: string
  onChange: (priority: Priority | '') => void
}

// PriorityControl sets how much the user wants to apply to a Job Listing
// (issue #280). Choosing "not set" clears it.
export default function PriorityControl({ value, disabled, label, onChange }: Props) {
  return (
    <Select
      value={value ?? NOT_SET}
      disabled={disabled}
      onValueChange={(next) => onChange(next === NOT_SET ? '' : (next as Priority))}
    >
      <SelectTrigger size="sm" className="w-44" aria-label={label ? `Priority for ${label}` : 'Priority'}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={NOT_SET}>Priority: not set</SelectItem>
        {PRIORITIES.map((p) => (
          <SelectItem key={p} value={p}>
            Priority: {PRIORITY_LABELS[p].toLowerCase()}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
