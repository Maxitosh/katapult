// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-create-transfer:p1

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

interface ClusterSelectorProps {
  clusters: string[]
  value: string
  onChange: (cluster: string) => void
  label: string
  disabled?: boolean
}

export function ClusterSelector({
  clusters,
  value,
  onChange,
  label,
  disabled = false,
}: ClusterSelectorProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-medium text-foreground">{label}</label>
      <Select value={value} onValueChange={(val) => { if (val) onChange(val) }} disabled={disabled}>
        <SelectTrigger className="w-full">
          <SelectValue placeholder="Select cluster..." />
        </SelectTrigger>
        <SelectContent>
          {clusters.map((cluster) => (
            <SelectItem key={cluster} value={cluster}>
              {cluster}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
