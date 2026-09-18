package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"mine-ventilation-network-simulator/backend/internal/constants"
	"mine-ventilation-network-simulator/backend/internal/dto"
	"mine-ventilation-network-simulator/backend/internal/model"
	"mine-ventilation-network-simulator/backend/internal/repository"
	"mine-ventilation-network-simulator/backend/pkg/api"
)

func fingerprintFixture() ([]model.VentilationNode, []model.AirwayEdge) {
	nodes := []model.VentilationNode{
		{ID: 2, Code: "WF", NodeType: string(constants.NodeTypeWorkface), RequiredAirflowM3S: 8, PressurePa: 500, Status: string(constants.NodeStatusActive)},
		{ID: 1, Code: "IN", NodeType: string(constants.NodeTypeIntake), PressurePa: 1000, Status: string(constants.NodeStatusActive)},
	}
	edges := []model.AirwayEdge{
		{ID: 2, Code: "E2", FromNodeID: 2, ToNodeID: 3, ResistanceNS2M8: 2, AreaM2: 5, MaxVelocityMS: 5, DoorState: string(constants.DoorStateOpen), Enabled: true},
		{ID: 1, Code: "E1", FromNodeID: 1, ToNodeID: 2, ResistanceNS2M8: 2, AreaM2: 5, MaxVelocityMS: 5, DoorState: string(constants.DoorStateOpen), Enabled: true, CriticalPath: true},
	}
	return nodes, edges
}

func findChange(changes []dto.FreshnessChange, entityType string, entityID uint) *dto.FreshnessChange {
	for i := range changes {
		if changes[i].EntityType == entityType && changes[i].EntityID == entityID {
			return &changes[i]
		}
	}
	return nil
}

func TestBuildNetworkFingerprintIsDeterministicAndOrderIndependent(t *testing.T) {
	nodes, edges := fingerprintFixture()
	first := buildNetworkFingerprint(nodes, edges)
	reversed := buildNetworkFingerprint(
		[]model.VentilationNode{nodes[1], nodes[0]},
		[]model.AirwayEdge{edges[1], edges[0]},
	)
	if first.Hash == "" || first.Hash != reversed.Hash {
		t.Fatalf("fingerprint hash must not depend on input order: %q vs %q", first.Hash, reversed.Hash)
	}
	again := buildNetworkFingerprint(nodes, edges)
	if first.Hash != again.Hash {
		t.Fatalf("fingerprint hash is not deterministic: %q vs %q", first.Hash, again.Hash)
	}
}

func TestEvaluateFreshnessStaysFreshWhenParametersMatch(t *testing.T) {
	nodes, edges := fingerprintFixture()
	baseline := mustJSON(buildNetworkFingerprint(nodes, edges))
	freshness := evaluateFreshness(baseline, nodes, edges, time.Now().UTC())
	if freshness.Stale || freshness.Status != string(constants.FreshnessStatusFresh) {
		t.Fatalf("expected fresh result, got %#v", freshness)
	}
	if len(freshness.Changes) != 0 {
		t.Fatalf("expected no changes, got %#v", freshness.Changes)
	}
}

func TestEvaluateFreshnessFlagsModifiedNodeAndEdgeFields(t *testing.T) {
	nodes, edges := fingerprintFixture()
	baseline := mustJSON(buildNetworkFingerprint(nodes, edges))
	nodes[1].PressurePa = 1200
	edges[1].DoorState = string(constants.DoorStateRegulate)
	freshness := evaluateFreshness(baseline, nodes, edges, time.Now().UTC())
	if !freshness.Stale || freshness.Status != string(constants.FreshnessStatusStale) {
		t.Fatalf("expected stale result, got %#v", freshness)
	}
	nodeChange := findChange(freshness.Changes, "ventilation_node", 1)
	if nodeChange == nil || nodeChange.Change != string(constants.FreshnessChangeModified) {
		t.Fatalf("expected modified node change, got %#v", freshness.Changes)
	}
	if len(nodeChange.Fields) != 1 || nodeChange.Fields[0] != "pressure_pa" {
		t.Fatalf("expected pressure_pa field diff, got %#v", nodeChange.Fields)
	}
	edgeChange := findChange(freshness.Changes, "airway_edge", 1)
	if edgeChange == nil || edgeChange.Change != string(constants.FreshnessChangeModified) {
		t.Fatalf("expected modified edge change, got %#v", freshness.Changes)
	}
	if len(edgeChange.Fields) != 1 || edgeChange.Fields[0] != "door_state" {
		t.Fatalf("expected door_state field diff, got %#v", edgeChange.Fields)
	}
}

func TestEvaluateFreshnessFlagsRemovedAndAddedObjects(t *testing.T) {
	nodes, edges := fingerprintFixture()
	baseline := mustJSON(buildNetworkFingerprint(nodes, edges))
	// 巷道 E2 被停用后不再进入启用集合，节点 IN 保持但新增一个启用节点。
	currentNodes := append([]model.VentilationNode{}, nodes...)
	currentNodes = append(currentNodes, model.VentilationNode{ID: 3, Code: "OUT", NodeType: string(constants.NodeTypeExhaust), Status: string(constants.NodeStatusActive)})
	currentEdges := edges[1:]
	freshness := evaluateFreshness(baseline, currentNodes, currentEdges, time.Now().UTC())
	if !freshness.Stale {
		t.Fatalf("expected stale result, got %#v", freshness)
	}
	removed := findChange(freshness.Changes, "airway_edge", 2)
	if removed == nil || removed.Change != string(constants.FreshnessChangeRemoved) || removed.Code != "E2" {
		t.Fatalf("expected removed edge change, got %#v", freshness.Changes)
	}
	added := findChange(freshness.Changes, "ventilation_node", 3)
	if added == nil || added.Change != string(constants.FreshnessChangeAdded) || added.Code != "OUT" {
		t.Fatalf("expected added node change, got %#v", freshness.Changes)
	}
}

func TestEvaluateFreshnessUnknownWithoutUsableFingerprint(t *testing.T) {
	nodes, edges := fingerprintFixture()
	for name, raw := range map[string]datatypes.JSON{
		"empty":   {},
		"null":    datatypes.JSON([]byte("null")),
		"corrupt": datatypes.JSON([]byte(`{"hash":`)),
	} {
		freshness := evaluateFreshness(raw, nodes, edges, time.Now().UTC())
		if freshness.Stale || freshness.Status != string(constants.FreshnessStatusUnknown) {
			t.Fatalf("%s: expected unknown non-stale result, got %#v", name, freshness)
		}
	}
}

func setupSimulationServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:freshness-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.VentilationNode{}, &model.AirwayEdge{}, &model.FanScenario{}, &model.SimulationRun{}, &model.AuditEvent{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedApprovedScenario(t *testing.T, db *gorm.DB) model.FanScenario {
	t.Helper()
	nodes := []model.VentilationNode{
		{Code: "IN", NodeType: string(constants.NodeTypeIntake), PressurePa: 1000, Status: string(constants.NodeStatusActive)},
		{Code: "WF", NodeType: string(constants.NodeTypeWorkface), RequiredAirflowM3S: 8, PressurePa: 500, Status: string(constants.NodeStatusActive)},
		{Code: "OUT", NodeType: string(constants.NodeTypeExhaust), PressurePa: 0, Status: string(constants.NodeStatusActive)},
	}
	if err := db.Create(&nodes).Error; err != nil {
		t.Fatalf("seed nodes: %v", err)
	}
	edges := []model.AirwayEdge{
		{Code: "E1", FromNodeID: nodes[0].ID, ToNodeID: nodes[1].ID, ResistanceNS2M8: 2, AreaM2: 5, MaxVelocityMS: 5, DoorState: string(constants.DoorStateOpen), Enabled: true, CriticalPath: true, Version: 1},
		{Code: "E2", FromNodeID: nodes[1].ID, ToNodeID: nodes[2].ID, ResistanceNS2M8: 2, AreaM2: 5, MaxVelocityMS: 5, DoorState: string(constants.DoorStateOpen), Enabled: true, CriticalPath: true, Version: 1},
	}
	if err := db.Create(&edges).Error; err != nil {
		t.Fatalf("seed edges: %v", err)
	}
	curve := datatypes.JSON([]byte(`[{"flow_m3s":0,"pressure_pa":900},{"flow_m3s":30,"pressure_pa":600},{"flow_m3s":60,"pressure_pa":300}]`))
	scenario := model.FanScenario{
		Name: "基准方案", Description: "新鲜度校验测试方案", FanCurveJSON: curve, OperatingMode: "normal",
		ScenarioStatus: string(constants.ScenarioStatusApproved), SolverTolerance: 0.02, MaxIterations: 120,
		Version: 1, CreatedBy: 1,
	}
	if err := db.Create(&scenario).Error; err != nil {
		t.Fatalf("seed scenario: %v", err)
	}
	return scenario
}

func newSimulationService(db *gorm.DB) *SimulationService {
	return NewSimulationService(
		repository.NewSimulationRunRepository(db),
		repository.NewFanScenarioRepository(db),
		repository.NewVentilationNodeRepository(db),
		repository.NewAirwayEdgeRepository(db),
	)
}

func TestStartStoresNetworkFingerprintAndConfirmFollowsFreshness(t *testing.T) {
	db := setupSimulationServiceDB(t)
	scenario := seedApprovedScenario(t, db)
	service := newSimulationService(db)
	actor := Actor{ID: 1, Email: "reviewer@mine.local", Name: "安全复核员", Role: string(constants.RoleReviewer)}

	started, err := service.Start(context.Background(), scenario.ID, actor)
	if err != nil {
		t.Fatalf("start simulation: %v", err)
	}
	if len(started.NetworkFingerprintJSON) == 0 {
		t.Fatal("start must persist the network fingerprint")
	}
	if started.Freshness == nil || started.Freshness.Stale || started.Freshness.Status != string(constants.FreshnessStatusFresh) {
		t.Fatalf("fresh run expected right after start, got %#v", started.Freshness)
	}

	// 参数一致时确认流程保持不变。
	confirmed, err := service.ConfirmRisks(context.Background(), started.ID, "复核通过，参数未变化", actor)
	if err != nil {
		t.Fatalf("confirm risks on fresh run: %v", err)
	}
	if confirmed.RiskConfirmedAt == nil {
		t.Fatal("confirmation timestamp must be recorded for fresh runs")
	}
}

func TestConfirmRisksRejectedAfterNetworkDriftButHistoryKept(t *testing.T) {
	db := setupSimulationServiceDB(t)
	scenario := seedApprovedScenario(t, db)
	service := newSimulationService(db)
	actor := Actor{ID: 1, Email: "reviewer@mine.local", Name: "安全复核员", Role: string(constants.RoleReviewer)}

	started, err := service.Start(context.Background(), scenario.ID, actor)
	if err != nil {
		t.Fatalf("start simulation: %v", err)
	}

	// 推演完成后修改启用节点的关键参数，指纹随之失效。
	var node model.VentilationNode
	if err := db.Where("code = ?", "IN").First(&node).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if err := db.Model(&node).Update("pressure_pa", node.PressurePa+250).Error; err != nil {
		t.Fatalf("drift node pressure: %v", err)
	}

	listed, _, _, _, err := service.List(context.Background(), dto.SimulationListQuery{})
	if err != nil {
		t.Fatalf("list simulations: %v", err)
	}
	if len(listed) != 1 || listed[0].Freshness == nil || !listed[0].Freshness.Stale {
		t.Fatalf("list must mark drifted run as stale, got %#v", listed)
	}
	change := findChange(listed[0].Freshness.Changes, "ventilation_node", node.ID)
	if change == nil || change.Change != string(constants.FreshnessChangeModified) || len(change.Fields) != 1 || change.Fields[0] != "pressure_pa" {
		t.Fatalf("list must point at the changed object, got %#v", listed[0].Freshness.Changes)
	}

	detail, err := service.Get(context.Background(), started.ID)
	if err != nil {
		t.Fatalf("get simulation: %v", err)
	}
	if detail.Freshness == nil || !detail.Freshness.Stale {
		t.Fatalf("detail must mark drifted run as stale, got %#v", detail.Freshness)
	}

	_, err = service.ConfirmRisks(context.Background(), started.ID, "尝试确认过期证据", actor)
	var appErr *api.AppError
	if !errors.As(err, &appErr) || appErr.Code != "SIMULATION_STALE" {
		t.Fatalf("expected SIMULATION_STALE conflict, got %v", err)
	}

	// 历史结果原样保留：状态、残差与未确认字段不被新鲜度校验修改。
	var stored model.SimulationRun
	if err := db.First(&stored, started.ID).Error; err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if stored.RunStatus != started.RunStatus || stored.Residual != started.Residual || stored.RiskConfirmedAt != nil {
		t.Fatalf("historical result must stay untouched, got %#v", stored)
	}

	// 重新发起同方案推演后指纹恢复一致，可以正常确认。
	rerun, err := service.Start(context.Background(), scenario.ID, actor)
	if err != nil {
		t.Fatalf("restart simulation: %v", err)
	}
	if rerun.Freshness == nil || rerun.Freshness.Stale {
		t.Fatalf("rerun against current parameters must be fresh, got %#v", rerun.Freshness)
	}
	if _, err := service.ConfirmRisks(context.Background(), rerun.ID, "重新推演后确认", actor); err != nil {
		t.Fatalf("confirm risks after rerun: %v", err)
	}
}
