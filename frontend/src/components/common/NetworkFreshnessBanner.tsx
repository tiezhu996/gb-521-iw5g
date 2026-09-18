import { Alert, Table } from 'antd';
import { History, PlusCircle, Trash2, Wrench } from 'lucide-react';
import type { ColumnsType } from 'antd/es/table';
import { useMemo } from 'react';
import type { NetworkChange } from '../../types/simulation';
import { describeChangeType, describeEntity, describeFieldDelta, describeChangedFields, networkChangeTypeLabels } from '../../utils/freshness';

const changeIcons = {
  modified: Wrench,
  added: PlusCircle,
  removed: Trash2,
};

interface FreshnessBannerProps {
  changes: NetworkChange[];
  compact?: boolean;
  action?: React.ReactNode;
}

export function NetworkFreshnessBanner({ changes, compact = false, action }: FreshnessBannerProps) {
  const columns: ColumnsType<NetworkChange> = useMemo(() => [
    {
      title: '变化对象', width: 200,
      render: (_, row) => {
        const Icon = changeIcons[row.change_type] ?? History;
        return <span className="stale-change-entity"><Icon size={15} />{describeEntity(row)}</span>;
      },
    },
    { title: '变化类型', width: 230, render: (_, row) => describeChangeType(row) },
    {
      title: '变化参数',
      render: (_, row) => {
        if (row.change_type !== 'modified') {
          return <span className="muted">{networkChangeTypeLabels[row.change_type]}</span>;
        }
        return (
          <ul className="stale-field-list">
            {(row.fields ?? []).map((field) => (
              <li key={field}>{describeFieldDelta(row, field)}</li>
            ))}
          </ul>
        );
      },
    },
  ], []);
  const count = changes.length;
  const nodeCount = changes.filter((change) => change.entity_type === 'ventilation_node').length;
  const edgeCount = changes.filter((change) => change.entity_type === 'airway_edge').length;
  const summary = [nodeCount > 0 ? `${nodeCount} 个通风节点` : '', edgeCount > 0 ? `${edgeCount} 条巷道` : ''].filter(Boolean).join('、');
  return (
    <Alert
      className="stale-alert"
      type="error"
      showIcon
      icon={<History size={19} />}
      message={<strong>本轮推演所依据的网络参数已过期：{summary}发生变化（共 {count} 项）</strong>}
      description={
        <>
          <p>历史计算结果与已有人工确认仍原样保留，但当前参数与推演时刻保存的指纹不一致。以下对象已变化，请勿再依据本轮证据进行联锁判断。</p>
          {!compact && (
            <Table<NetworkChange>
              rowKey={(row) => `${row.entity_type}-${row.entity_id}-${row.change_type}`}
              columns={columns}
              dataSource={changes}
              size="small"
              pagination={false}
              scroll={{ x: 640 }}
            />
          )}
          {compact && (
            <ul className="stale-field-list">
              {changes.map((change) => (
                <li key={`${change.entity_type}-${change.entity_id}-${change.change_type}`}>
                  <strong>{describeEntity(change)}</strong>：{describeChangeType(change)}
                  {describeChangedFields(change) ? `（${describeChangedFields(change)}）` : ''}
                </li>
              ))}
            </ul>
          )}
          {action}
        </>
      }
    />
  );
}
