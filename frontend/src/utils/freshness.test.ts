import { describe, expect, it } from 'vitest';
import { describeFreshnessChange } from './freshness';

describe('freshness change descriptions', () => {
  it('describes modified objects with translated field labels', () => {
    expect(describeFreshnessChange({ entity_type: 'airway_edge', entity_id: 7, code: 'AW-101', change: 'modified', fields: ['resistance_ns2m8', 'door_state'] }))
      .toBe('巷道 AW-101 参数变化（风阻、风门状态）');
  });

  it('describes added and removed objects without field lists', () => {
    expect(describeFreshnessChange({ entity_type: 'ventilation_node', entity_id: 3, code: 'WF-07', change: 'added' }))
      .toBe('节点 WF-07 新增启用');
    expect(describeFreshnessChange({ entity_type: 'airway_edge', entity_id: 9, code: '', change: 'removed' }))
      .toBe('巷道 #9 停用或移除');
  });
});
