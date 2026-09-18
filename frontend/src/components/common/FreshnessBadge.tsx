import { Alert } from 'antd';
import { CheckCircle2, CircleDot, History } from 'lucide-react';
import type { NetworkFreshness } from '../../types/simulation';
import { describeFreshnessChange } from '../../utils/freshness';

export function FreshnessBadge({ freshness }: { freshness?: NetworkFreshness }) {
  if (!freshness || freshness.status === 'unknown') {
    return <span className="status-badge status-neutral"><CircleDot size={14} aria-hidden="true" />未校验</span>;
  }
  if (freshness.stale) {
    return <span className="status-badge status-danger"><History size={14} aria-hidden="true" />已过期</span>;
  }
  return <span className="status-badge status-success"><CheckCircle2 size={14} aria-hidden="true" />参数一致</span>;
}

export function FreshnessChangesAlert({ freshness, hint }: { freshness?: NetworkFreshness; hint?: string }) {
  if (!freshness?.stale) return null;
  return (
    <Alert
      className="section-alert"
      type="warning"
      showIcon
      message="网络关键参数已变化，该轮推演结果已过期"
      description={
        <>
          <ul className="freshness-changes">
            {freshness.changes.map((change) => (
              <li key={`${change.entity_type}-${change.entity_id}-${change.change}`}>{describeFreshnessChange(change)}</li>
            ))}
          </ul>
          {hint && <p className="freshness-hint">{hint}</p>}
        </>
      }
    />
  );
}
