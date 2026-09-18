import type { FreshnessChange } from '../types/simulation';

export const freshnessEntityLabels: Record<string, string> = {
  ventilation_node: '节点',
  airway_edge: '巷道',
};

export const freshnessChangeLabels: Record<string, string> = {
  added: '新增启用',
  removed: '停用或移除',
  modified: '参数变化',
};

export const freshnessFieldLabels: Record<string, string> = {
  code: '编号',
  node_type: '节点类型',
  elevation_m: '标高',
  required_airflow_m3s: '最低需风量',
  pressure_pa: '边界压力',
  from_node_id: '起点节点',
  to_node_id: '终点节点',
  resistance_ns2m8: '风阻',
  area_m2: '断面积',
  max_velocity_ms: '风速上限',
  door_state: '风门状态',
  critical_path: '关键路径标记',
};

export function describeFreshnessChange(change: FreshnessChange): string {
  const entity = freshnessEntityLabels[change.entity_type] ?? change.entity_type;
  const action = freshnessChangeLabels[change.change] ?? change.change;
  const fields = change.fields?.length
    ? `（${change.fields.map((field) => freshnessFieldLabels[field] ?? field).join('、')}）`
    : '';
  return `${entity} ${change.code || `#${change.entity_id}`} ${action}${fields}`;
}
