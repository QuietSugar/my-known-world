import type { NodeType } from './api'

export function antdColorForType(type: NodeType): string {
  switch (type) {
    case 'Machine': return 'green'
    case 'Hardware': return 'blue'
    case 'SoftwareProject': return 'gold'
    case 'SoftwareInstance': return 'cyan'
    case 'AccountCredential': return 'magenta'
    case 'Document': return 'default'
    case 'Runbook': return 'orange'
    case 'WorkItem': return 'purple'
    case 'Network': return 'geekblue'
    case 'ProvisionalObject': return 'volcano'
    case 'RunningPeriod': return 'lime'
    case 'Product': return 'blue'
    case 'Self': return 'purple'
    default: return 'default'
  }
}
