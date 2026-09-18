import type { NetworkChange } from '../types/simulation';

export const networkEntityLabels: Record<string, string> = {
  ventilation_node: '通风节点',
  airway_edge: '巷道',
};

export const networkChangeTypeLabels: Record<string, string> = {
  modified: '参数已修改',
  added: '新增（推演时不在网络中）',
  removed: '已停用或删除（推演时处于启用状态）',
};

export const networkFieldLabels: Record<string, string> = {
  node_type: '节点类型',
  elevation_m: '标高 (m)',
  required_airflow_m3s: '最低需风量 (m³/s)',
  pressure_pa: '边界压力 (Pa)',
  status: '节点状态',
  from_node_id: '起始节点',
  to_node_id: '终止节点',
  resistance_ns2m8: '风阻 (N·s²/m⁸)',
  area_m2: '断面面积 (m²)',
  max_velocity_ms: '允许最高风速 (m/s)',
  door_state: '风门状态',
  enabled: '启用状态',
  critical_path: '关键路径标记',
};

export const doorStateLabels: Record<string, string> = {
  open: '开启',
  closed: '关闭',
  regulating: '调节',
};

export const nodeStatusLabels: Record<string, string> = {
  active: '启用',
  inactive: '停用',
  blocked: '阻断',
};

export function describeEntity(change: NetworkChange): string {
  const kind = networkEntityLabels[change.entity_type] ?? change.entity_type;
  return `${kind} ${change.code || `#${change.entity_id}`}`;
}

export function describeChangeType(change: NetworkChange): string {
  return networkChangeTypeLabels[change.change_type] ?? change.change_type;
}

export function describeField(field: string): string {
  return networkFieldLabels[field] ?? field;
}

function formatValue(entityType: string, field: string, value: unknown): string {
  if (value === null || value === undefined || value === '') return '—';
  if (field === 'door_state') return doorStateLabels[String(value)] ?? String(value);
  if (field === 'status') return nodeStatusLabels[String(value)] ?? String(value);
  if (field === 'enabled') return value ? '启用' : '停用';
  if (field === 'critical_path') return value ? '关键路径' : '普通路径';
  if (field === 'node_type') return String(value);
  if (typeof value === 'number') {
    return Number.isInteger(value) ? String(value) : String(Math.round(value * 10000) / 10000);
  }
  if (entityType === 'airway_edge' && (field === 'from_node_id' || field === 'to_node_id')) {
    return `节点 #${value}`;
  }
  return String(value);
}

export function describeChangedFields(change: NetworkChange): string {
  if (change.change_type !== 'modified' || !change.fields?.length) return '';
  return change.fields.map(describeField).join('、');
}

export function describeFieldDelta(change: NetworkChange, field: string): string {
  const before = formatValue(change.entity_type, field, change.before?.[field]);
  const after = formatValue(change.entity_type, field, change.after?.[field]);
  return `${describeField(field)}：${before} → ${after}`;
}

export function summarizeNetworkChanges(changes: NetworkChange[]): string {
  const nodes = changes.filter((change) => change.entity_type === 'ventilation_node').length;
  const edges = changes.filter((change) => change.entity_type === 'airway_edge').length;
  const parts: string[] = [];
  if (nodes > 0) parts.push(`${nodes} 个节点`);
  if (edges > 0) parts.push(`${edges} 条巷道`);
  return parts.join('、');
}
