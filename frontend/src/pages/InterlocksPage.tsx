import { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Select, message } from 'antd';
import { CheckCircle2, ShieldAlert } from 'lucide-react';
import { Link } from 'react-router-dom';
import { ConfirmActionDialog } from '../components/common/ConfirmActionDialog';
import { NetworkFreshnessBanner } from '../components/common/NetworkFreshnessBanner';
import { PageHeader } from '../components/common/PageHeader';
import { RiskEvidenceTable } from '../components/common/RiskEvidenceTable';
import { StatusBadge } from '../components/common/StatusBadge';
import { useAuth } from '../hooks/useAuth';
import { useSimulationPolling } from '../hooks/useSimulationPolling';
import { useEdgeStore } from '../stores/edgeStore';
import { useSimulationStore } from '../stores/simulationStore';
import { reportError } from '../utils/errors';
import { formatDateTime } from '../utils/format';

export function InterlocksPage() {
  const { hasRole } = useAuth();
  const { edges, load: loadEdges } = useEdgeStore();
  const { runs, selected, select, confirm } = useSimulationStore();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [note, setNote] = useState('');
  const [busy, setBusy] = useState(false);
  useSimulationPolling(true, 9000);
  useEffect(() => { loadEdges().catch(reportError); }, [loadEdges]);
  const runsWithEvidence = useMemo(() => runs.filter((run) => (run.risk_flags_json?.length ?? 0) > 0), [runs]);
  const staleRunIds = useMemo(() => new Set(runsWithEvidence.filter((run) => !run.network_fresh).map((run) => run.id)), [runsWithEvidence]);
  useEffect(() => { if (!selected && runsWithEvidence[0]) void select(runsWithEvidence[0].id); }, [runsWithEvidence, select, selected]);
  const selectedEdgeIds = new Set((selected?.risk_flags_json ?? []).filter((risk) => risk.entity_type === 'airway_edge').map((risk) => risk.entity_id));
  const affectedEdges = edges.filter((edge) => selectedEdgeIds.has(edge.id));
  const selectedStale = Boolean(selected && !selected.network_fresh);
  const canConfirm = Boolean(selected && !selected.risk_confirmed_at && !selectedStale && hasRole('reviewer', 'admin'));
  const doConfirm = async () => {
    if (!selected || selectedStale) return;
    setBusy(true);
    try { await confirm(selected.id, note); message.success('风险证据已由当前复核人员确认'); setDialogOpen(false); setNote(''); } catch (error) { reportError(error, '风险确认未完成'); } finally { setBusy(false); }
  };
  return (
    <div className="page">
      <PageHeader eyebrow="规则引擎 / 人工确认" title="联锁风险证据" meta={<><span>{runsWithEvidence.length} 次运行触发规则</span><span>{edges.filter((edge) => edge.critical_path).length} 条关键路径</span><span>确认不等于现场执行授权</span></>} />
      <Alert className="section-alert" type="error" showIcon message="风险证据必须结合现场规程人工复核，系统不会下发任何控制命令" />
      {staleRunIds.size > 0 && <Alert className="section-alert" type="warning" showIcon message={`有 ${staleRunIds.size} 轮历史推演的网络参数已过期，列表中已标记；过期证据不能在本页确认，必须使用同一方案重新发起推演。`} />}
      <section className="interlock-selector"><label htmlFor="risk-run">推演记录</label><Select id="risk-run" value={selected?.id} onChange={(id) => select(id).catch(reportError)} options={runsWithEvidence.map((run) => ({ value: run.id, label: `#${run.id} · ${run.scenario?.name ?? `方案 ${run.scenario_id}`} · ${formatDateTime(run.started_at)}${!run.network_fresh ? ' · 已过期' : ''}` }))} placeholder="暂无触发风险规则的运行" /></section>
      {selected ? <>
        {selectedStale && <NetworkFreshnessBanner changes={selected.network_changes ?? []} action={<p className="stale-action-hint">联锁页不得确认过期证据。请通知工程师/管理员保持方案「{selected.scenario?.name ?? `#${selected.scenario_id}`}」不变，<Link to="/simulations">前往离线推演工作台</Link> 重新发起同方案推演，再对新一轮运行进行人工确认。</p>} />}
        <section className="risk-summary"><div><ShieldAlert size={23} /><span>规则触发</span><strong>{selected.risk_flags_json?.length ?? 0}</strong></div><div><span>关联巷道</span><strong>{affectedEdges.length}</strong></div><div><span>人工状态</span>{selected.risk_confirmed_at ? <StatusBadge status="confirmed" /> : selectedStale ? <StatusBadge status="stale" /> : <strong>待确认</strong>}</div><Button type="primary" icon={<CheckCircle2 size={17} />} disabled={!canConfirm} onClick={() => setDialogOpen(true)}>{selected.risk_confirmed_at ? '证据已确认' : selectedStale ? '证据已过期，禁止确认' : '确认风险证据'}</Button></section>
        <section className="workspace-section"><div className="section-heading"><div><span className="section-index">01</span><h2>规则、证据值与阈值</h2></div></div><RiskEvidenceTable risks={selected.risk_flags_json ?? []} /></section>
        <section className="workspace-section affected-list"><div className="section-heading"><div><span className="section-index">02</span><h2>受影响巷道</h2></div></div>{affectedEdges.length ? affectedEdges.map((edge) => <div key={edge.id}><strong>{edge.code}</strong><span>{edge.from_node?.code ?? edge.from_node_id} → {edge.to_node?.code ?? edge.to_node_id}</span><span>{edge.critical_path ? '关键路径' : '普通路径'}</span><span>风门：{edge.door_state}</span></div>) : <p className="muted">当前证据未直接关联巷道边。</p>}</section>
      </> : <div className="empty-state"><ShieldAlert size={28} /><h2>暂无风险证据</h2><p>完成已批准方案的推演后，触发的规则会出现在这里。</p></div>}
      <ConfirmActionDialog open={dialogOpen} title="确认已复核风险证据" consequence="此操作只记录你已查看当前证据，不会改变规则判定，也不会向现场设备发送指令。确认记录不可通过普通 API 删除。" confirmLabel="记录人工确认" noteLabel="复核说明" note={note} requireNote busy={busy} onNoteChange={setNote} onCancel={() => { setDialogOpen(false); setNote(''); }} onConfirm={() => void doConfirm()} />
    </div>
  );
}
