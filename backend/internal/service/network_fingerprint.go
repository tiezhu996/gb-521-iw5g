package service

import (
	"context"
	"encoding/json"
	"reflect"

	"gorm.io/datatypes"

	"mine-ventilation-network-simulator/backend/internal/model"
)

const networkFingerprintVersion = "network-params-v1"

// buildNetworkFingerprint 对当前已启用节点与已启用巷道的关键参数生成指纹。
// 调用方需保证 nodes、edges 取自 AllActive / AllEnabled（已按 id 升序）。
func buildNetworkFingerprint(nodes []model.VentilationNode, edges []model.AirwayEdge) model.NetworkFingerprint {
	fingerprint := model.NetworkFingerprint{
		Version: networkFingerprintVersion,
		Nodes:   make([]model.NodeFingerprint, 0, len(nodes)),
		Edges:   make([]model.EdgeFingerprint, 0, len(edges)),
	}
	for _, node := range nodes {
		fingerprint.Nodes = append(fingerprint.Nodes, model.NodeFingerprint{
			ID:                 node.ID,
			Code:               node.Code,
			NodeType:           node.NodeType,
			ElevationM:         node.ElevationM,
			RequiredAirflowM3S: node.RequiredAirflowM3S,
			PressurePa:         node.PressurePa,
			Status:             node.Status,
		})
	}
	for _, edge := range edges {
		fingerprint.Edges = append(fingerprint.Edges, model.EdgeFingerprint{
			ID:              edge.ID,
			Code:            edge.Code,
			FromNodeID:      edge.FromNodeID,
			ToNodeID:        edge.ToNodeID,
			ResistanceNS2M8: edge.ResistanceNS2M8,
			AreaM2:          edge.AreaM2,
			MaxVelocityMS:   edge.MaxVelocityMS,
			DoorState:       edge.DoorState,
			Enabled:         edge.Enabled,
			CriticalPath:    edge.CriticalPath,
		})
	}
	return fingerprint
}

func mustFingerprintJSON(fingerprint model.NetworkFingerprint) datatypes.JSON {
	data, err := json.Marshal(fingerprint)
	if err != nil {
		return datatypes.JSON([]byte(`null`))
	}
	return datatypes.JSON(data)
}

// loadNetworkFingerprint 解析推演保存时的网络指纹。空指纹（历史数据）返回 ok=false。
func loadNetworkFingerprint(raw datatypes.JSON) (model.NetworkFingerprint, bool) {
	var fingerprint model.NetworkFingerprint
	if len(raw) == 0 || string(raw) == "null" {
		return fingerprint, false
	}
	if err := json.Unmarshal(raw, &fingerprint); err != nil {
		return fingerprint, false
	}
	if fingerprint.Version == "" || (len(fingerprint.Nodes) == 0 && len(fingerprint.Edges) == 0) {
		return fingerprint, false
	}
	return fingerprint, true
}

// diffNetworkFingerprint 比较推演时刻指纹与当前启用网络，返回全部参数差异。
func diffNetworkFingerprint(stored model.NetworkFingerprint, nodes []model.VentilationNode, edges []model.AirwayEdge) []model.NetworkChange {
	current := buildNetworkFingerprint(nodes, edges)
	changes := make([]model.NetworkChange, 0)

	storedNodes := make(map[uint]model.NodeFingerprint, len(stored.Nodes))
	for _, node := range stored.Nodes {
		storedNodes[node.ID] = node
	}
	currentNodes := make(map[uint]model.NodeFingerprint, len(current.Nodes))
	for _, node := range current.Nodes {
		currentNodes[node.ID] = node
	}
	for _, old := range stored.Nodes {
		now, ok := currentNodes[old.ID]
		if !ok {
			changes = append(changes, model.NetworkChange{
				EntityType: "ventilation_node", EntityID: old.ID, Code: old.Code,
				ChangeType: "removed", Before: fingerprintMap(old),
			})
			continue
		}
		if fields := changedFingerprintFields(old, now); len(fields) > 0 {
			changes = append(changes, model.NetworkChange{
				EntityType: "ventilation_node", EntityID: old.ID, Code: old.Code,
				ChangeType: "modified", Fields: fields,
				Before: fingerprintMap(old), After: fingerprintMap(now),
			})
		}
	}
	for _, now := range current.Nodes {
		if _, ok := storedNodes[now.ID]; !ok {
			changes = append(changes, model.NetworkChange{
				EntityType: "ventilation_node", EntityID: now.ID, Code: now.Code,
				ChangeType: "added", After: fingerprintMap(now),
			})
		}
	}

	storedEdges := make(map[uint]model.EdgeFingerprint, len(stored.Edges))
	for _, edge := range stored.Edges {
		storedEdges[edge.ID] = edge
	}
	currentEdges := make(map[uint]model.EdgeFingerprint, len(current.Edges))
	for _, edge := range current.Edges {
		currentEdges[edge.ID] = edge
	}
	for _, old := range stored.Edges {
		now, ok := currentEdges[old.ID]
		if !ok {
			changes = append(changes, model.NetworkChange{
				EntityType: "airway_edge", EntityID: old.ID, Code: old.Code,
				ChangeType: "removed", Before: fingerprintMap(old),
			})
			continue
		}
		if fields := changedFingerprintFields(old, now); len(fields) > 0 {
			changes = append(changes, model.NetworkChange{
				EntityType: "airway_edge", EntityID: old.ID, Code: old.Code,
				ChangeType: "modified", Fields: fields,
				Before: fingerprintMap(old), After: fingerprintMap(now),
			})
		}
	}
	for _, now := range current.Edges {
		if _, ok := storedEdges[now.ID]; !ok {
			changes = append(changes, model.NetworkChange{
				EntityType: "airway_edge", EntityID: now.ID, Code: now.Code,
				ChangeType: "added", After: fingerprintMap(now),
			})
		}
	}
	return changes
}

// changedFingerprintFields 比较同 ID 对象，返回发生变化的参数字段名。
// code 只用于标识对象，不参与变化判定（id 不变则 code 视为同一对象的展示名）。
func changedFingerprintFields(before, after any) []string {
	beforeMap := fingerprintMap(before)
	afterMap := fingerprintMap(after)
	fields := make([]string, 0)
	for key, oldValue := range beforeMap {
		if key == "id" || key == "code" {
			continue
		}
		if !reflect.DeepEqual(oldValue, afterMap[key]) {
			fields = append(fields, key)
		}
	}
	return fields
}

func fingerprintMap(value any) map[string]any {
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	result := map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]any{}
	}
	return result
}

// evaluateNetworkFreshness 判断一次推演是否仍基于最新参数。
// 没有指纹的历史记录按“参数一致”处理，保持原有流程不变。
func evaluateNetworkFreshness(storedRaw datatypes.JSON, nodes []model.VentilationNode, edges []model.AirwayEdge) (bool, []model.NetworkChange) {
	stored, ok := loadNetworkFingerprint(storedRaw)
	if !ok {
		return true, nil
	}
	changes := diffNetworkFingerprint(stored, nodes, edges)
	return len(changes) == 0, changes
}

// currentEnabledNetwork 一次性读取当前启用节点与启用巷道。
func (s *SimulationService) currentEnabledNetwork(ctx context.Context) ([]model.VentilationNode, []model.AirwayEdge, error) {
	nodes, err := s.nodes.AllActive(ctx)
	if err != nil {
		return nil, nil, err
	}
	edges, err := s.edges.AllEnabled(ctx)
	if err != nil {
		return nil, nil, err
	}
	return nodes, edges, nil
}

func annotateRun(run *model.SimulationRun, nodes []model.VentilationNode, edges []model.AirwayEdge) *model.SimulationRunView {
	fresh, changes := evaluateNetworkFreshness(run.NetworkFingerprintJSON, nodes, edges)
	return &model.SimulationRunView{SimulationRun: run, NetworkFresh: fresh, NetworkChanges: changes}
}
