// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-create-transfer:p1

import type { PVCInfo } from "@/api/types"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

interface PVCSelectorProps {
  pvcs: PVCInfo[]
  value: string
  onChange: (pvcName: string) => void
  label: string
  disabled?: boolean
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const size = bytes / Math.pow(1024, i)
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function PVCSelector({
  pvcs,
  value,
  onChange,
  label,
  disabled = false,
}: PVCSelectorProps) {
  const isDisabled = disabled || pvcs.length === 0

  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-medium text-foreground">{label}</label>
      <Select value={value} onValueChange={(val) => { if (val) onChange(val) }} disabled={isDisabled}>
        <SelectTrigger className="w-full">
          <SelectValue
            placeholder={
              pvcs.length === 0 ? "No PVCs available" : "Select PVC..."
            }
          />
        </SelectTrigger>
        <SelectContent>
          {pvcs.map((pvc) => (
            <SelectItem key={pvc.pvc_name} value={pvc.pvc_name}>
              <span className="flex items-center gap-2">
                <span className="font-medium">{pvc.pvc_name}</span>
                <span className="text-muted-foreground">
                  {formatBytes(pvc.size_bytes)}
                </span>
                <span className="text-muted-foreground">
                  {pvc.storage_class}
                </span>
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
