package service

import (
	"testing"

	"mine-ventilation-network-simulator/backend/internal/constants"
	"mine-ventilation-network-simulator/backend/internal/model"
)

func fingerprintTestNetwork() ([]model.VentilationNode, []model.AirwayEdge) {
	nodes := []model.VentilationNode{
		{ID: 1, Code: "IN", NodeType: string(constants.NodeTypeIntake), Status: string(constants.NodeStatusActive), ElevationM: 12, PressurePa: 1000},
		{ID: 2, Code: "WF", NodeType: string(constants.NodeTypeWorkface), Status: string(constants.NodeStatusActive), ElevationM: -100, RequiredAirflowM3S: 8, PressurePa: 500},
		{ID: 3, Code: "OUT", NodeType: string(constants.NodeTypeExhaust), Status: string(constants.NodeStatusActive), ElevationM: 6, PressurePa: 0},
	}
	edges := []model.AirwayEdge{
		{ID: 1, Code: "E1", FromNodeID: 1, ToNodeID: 2, ResistanceNS2M8: 2, AreaM2: 5, MaxVelocityMS: 5, DoorState: string(constants.DoorStateOpen), Enabled: true, CriticalPath: true},
		{ID: 2, Code: "E2", FromNodeID: 2, ToNodeID: 3, ResistanceNS2M8: 2, AreaM2: 5, MaxVelocityMS: 5, DoorState: string(constants.DoorStateOpen), Enabled: true, CriticalPath: true},
	}
	return nodes, edges
}

func TestNetworkFingerprintFreshWhenParametersUnchanged(t *testing.T) {
	nodes, edges := fingerprintTestNetwork()
	stored := buildNetworkFingerprint(nodes, edges)
	if changes := diffNetworkFingerprint(stored, nodes, edges); len(changes) != 0 {
		t.Fatalf("identical network should have no changes, got %#v", changes)
	}
}

func TestNetworkFingerprintDetectsModifiedNodeAndEdge(t *testing.T) {
	nodes, edges := fingerprintTestNetwork()
	stored := buildNetworkFingerprint(nodes, edges)

	currentNodes := append([]model.VentilationNode(nil), nodes...)
	currentNodes[1].RequiredAirflowM3S = 12
	currentNodes[1].PressurePa = 460
	currentEdges := append([]model.AirwayEdge(nil), edges...)
	currentEdges[0].ResistanceNS2M8 = 3.5
	currentEdges[0].DoorState = string(constants.DoorStateClosed)

	changes := diffNetworkFingerprint(stored, currentNodes, currentEdges)
	if len(changes) != 2 {
		t.Fatalf("expected 2 modified objects, got %#v", changes)
	}
	changed := map[string][]string{}
	for _, change := range changes {
		if change.ChangeType != "modified" {
			t.Fatalf("expected modified change, got %s for %s #%d", change.ChangeType, change.EntityType, change.EntityID)
		}
		changed[change.EntityType] = change.Fields
	}
	assertContainsField(t, changed["ventilation_node"], "required_airflow_m3s")
	assertContainsField(t, changed["ventilation_node"], "pressure_pa")
	assertContainsField(t, changed["airway_edge"], "resistance_ns2m8")
	assertContainsField(t, changed["airway_edge"], "door_state")
}

func TestNetworkFingerprintDetectsNodeDisabledAndEdgeDisabled(t *testing.T) {
	nodes, edges := fingerprintTestNetwork()
	stored := buildNetworkFingerprint(nodes, edges)

	// 节点从启用集合中消失（停用）；巷道 enabled=false 也离开 AllEnabled 集合
	currentNodes := []model.VentilationNode{nodes[0], nodes[2]}
	currentEdges := []model.AirwayEdge{edges[1]}
	changes := diffNetworkFingerprint(stored, currentNodes, currentEdges)

	removed := map[string]uint{}
	for _, change := range changes {
		if change.ChangeType != "removed" {
			t.Fatalf("expected removed change, got %#v", change)
		}
		removed[change.EntityType] = change.EntityID
	}
	if removed["ventilation_node"] != 2 || removed["airway_edge"] != 1 {
		t.Fatalf("unexpected removed objects: %#v", removed)
	}
}

func TestNetworkFingerprintDetectsAddedNodeAndEdge(t *testing.T) {
	nodes, edges := fingerprintTestNetwork()
	stored := buildNetworkFingerprint(nodes[:2], edges[:1])

	changes := diffNetworkFingerprint(stored, nodes, edges)
	if len(changes) != 2 {
		t.Fatalf("expected 2 added objects, got %#v", changes)
	}
	for _, change := range changes {
		if change.ChangeType != "added" {
			t.Fatalf("expected added change, got %#v", change)
		}
	}
}

func TestLoadNetworkFingerprintTreatsLegacyRunAsFresh(t *testing.T) {
	// 历史推演没有指纹列数据：无法判定过期，按参数一致处理，保持原流程不变。
	if _, ok := loadNetworkFingerprint(nil); ok {
		t.Fatal("empty fingerprint should be reported as absent")
	}
	nodes, edges := fingerprintTestNetwork()
	if _, ok := loadNetworkFingerprint(mustFingerprintJSON(buildNetworkFingerprint(nodes, edges))); !ok {
		t.Fatal("valid fingerprint should be reported as present")
	}
}

func assertContainsField(t *testing.T, fields []string, want string) {
	t.Helper()
	for _, field := range fields {
		if field == want {
			return
		}
	}
	t.Fatalf("field %s missing from %v", want, fields)
}
