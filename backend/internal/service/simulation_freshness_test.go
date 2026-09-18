package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"mine-ventilation-network-simulator/backend/internal/constants"
	"mine-ventilation-network-simulator/backend/internal/dto"
	"mine-ventilation-network-simulator/backend/internal/model"
	"mine-ventilation-network-simulator/backend/internal/repository"
	"mine-ventilation-network-simulator/backend/pkg/api"
)

func newFreshnessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.VentilationNode{}, &model.AirwayEdge{}, &model.FanScenario{}, &model.SimulationRun{}, &model.AuditEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedFreshnessFixture(t *testing.T, db *gorm.DB) ([]model.VentilationNode, []model.AirwayEdge, model.FanScenario) {
	t.Helper()
	ctx := context.Background()
	nodeRepo := repository.NewVentilationNodeRepository(db)
	edgeRepo := repository.NewAirwayEdgeRepository(db)
	nodes := []model.VentilationNode{
		{Code: "IN", NodeType: string(constants.NodeTypeIntake), Status: string(constants.NodeStatusActive), ElevationM: 12, PressurePa: 1000},
		{Code: "WF", NodeType: string(constants.NodeTypeWorkface), Status: string(constants.NodeStatusActive), ElevationM: -100, RequiredAirflowM3S: 8, PressurePa: 500},
		{Code: "OUT", NodeType: string(constants.NodeTypeExhaust), Status: string(constants.NodeStatusActive), ElevationM: 6},
	}
	for i := range nodes {
		if err := nodeRepo.Create(ctx, &nodes[i], repository.AuditRecord{}); err != nil {
			t.Fatalf("create node: %v", err)
		}
	}
	edges := []model.AirwayEdge{
		{Code: "E1", FromNodeID: nodes[0].ID, ToNodeID: nodes[1].ID, ResistanceNS2M8: 2, AreaM2: 5, MaxVelocityMS: 5, DoorState: string(constants.DoorStateOpen), Enabled: true, CriticalPath: true},
		{Code: "E2", FromNodeID: nodes[1].ID, ToNodeID: nodes[2].ID, ResistanceNS2M8: 2, AreaM2: 5, MaxVelocityMS: 5, DoorState: string(constants.DoorStateOpen), Enabled: true, CriticalPath: true},
	}
	for i := range edges {
		if err := edgeRepo.Create(ctx, &edges[i], repository.AuditRecord{}); err != nil {
			t.Fatalf("create edge: %v", err)
		}
	}
	curveJSON, _ := json.Marshal([]dto.FanCurvePoint{{FlowM3S: 0, PressurePa: 900}, {FlowM3S: 30, PressurePa: 600}, {FlowM3S: 60, PressurePa: 300}})
	scenario := model.FanScenario{
		Name: "freshness", Description: "freshness test scenario", FanCurveJSON: datatypes.JSON(curveJSON), OperatingMode: "normal",
		ScenarioStatus: string(constants.ScenarioStatusApproved), SolverTolerance: 0.02, MaxIterations: 120, Version: 1,
		CreatedBy: 1,
	}
	if err := db.Create(&scenario).Error; err != nil {
		t.Fatalf("create scenario: %v", err)
	}
	return nodes, edges, scenario
}

func newSimulationService(db *gorm.DB) *SimulationService {
	return NewSimulationService(
		repository.NewSimulationRunRepository(db),
		repository.NewFanScenarioRepository(db),
		repository.NewVentilationNodeRepository(db),
		repository.NewAirwayEdgeRepository(db),
	)
}

func TestConfirmRisksBlockedWhenNetworkChanged(t *testing.T) {
	db := newFreshnessTestDB(t)
	svc := newSimulationService(db)
	ctx := context.Background()
	_, _, scenario := seedFreshnessFixture(t, db)
	actor := Actor{ID: 9, Email: "reviewer@mine.local", Name: "复核员", Role: string(constants.RoleReviewer)}

	run, err := svc.Start(ctx, scenario.ID, actor)
	if err != nil {
		t.Fatalf("start simulation: %v", err)
	}
	if !run.NetworkFresh || len(run.NetworkChanges) != 0 {
		t.Fatalf("new run should be fresh, got fresh=%v changes=%#v", run.NetworkFresh, run.NetworkChanges)
	}
	if len(run.NetworkFingerprintJSON) == 0 {
		t.Fatal("new run must persist a network fingerprint")
	}

	// 网络参数一致：可以确认（先构造一条有风险证据的运行）
	detail, err := svc.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if !detail.NetworkFresh {
		t.Fatalf("unchanged network must stay fresh: %#v", detail.NetworkChanges)
	}

	// 修改工作面关键参数：历史推演立即过期，已有确认前先直接验证拦截
	if err := db.Model(&model.VentilationNode{}).Where("code = ?", "WF").
		Update("required_airflow_m3_s", 30).Error; err != nil {
		t.Fatalf("mutate node: %v", err)
	}
	stale, err := svc.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("get stale run: %v", err)
	}
	if stale.NetworkFresh || len(stale.NetworkChanges) != 1 {
		t.Fatalf("expected stale with one change, got fresh=%v changes=%#v", stale.NetworkFresh, stale.NetworkChanges)
	}
	if stale.NetworkChanges[0].ChangeType != "modified" || stale.NetworkChanges[0].EntityType != "ventilation_node" {
		t.Fatalf("unexpected change descriptor: %#v", stale.NetworkChanges[0])
	}

	_, err = svc.ConfirmRisks(ctx, run.ID, "已结合现场规程复核", actor)
	var appErr *api.AppError
	if !errors.As(err, &appErr) || appErr.Code != "SIMULATION_NETWORK_STALE" {
		t.Fatalf("expected SIMULATION_NETWORK_STALE conflict, got %#v", err)
	}
	if appErr.Details == nil {
		t.Fatal("stale conflict must identify the changed objects in details")
	}

	// 历史结果原样保留：记录本身未被删除或改写
	var preserved model.SimulationRun
	if err := db.First(&preserved, run.ID).Error; err != nil {
		t.Fatalf("historical run should be preserved: %v", err)
	}
	if len(preserved.NetworkFingerprintJSON) == 0 {
		t.Fatal("preserved run fingerprint was altered")
	}
}

func TestLegacyRunWithoutFingerprintStillConfirmable(t *testing.T) {
	db := newFreshnessTestDB(t)
	svc := newSimulationService(db)
	ctx := context.Background()
	_, _, scenario := seedFreshnessFixture(t, db)
	actor := Actor{ID: 9, Role: string(constants.RoleReviewer)}

	run, err := svc.Start(ctx, scenario.ID, actor)
	if err != nil {
		t.Fatalf("start simulation: %v", err)
	}
	// 模拟指纹特性上线前的历史数据
	if err := db.Model(&model.SimulationRun{}).Where("id = ?", run.ID).
		Update("network_fingerprint_json", nil).Error; err != nil {
		t.Fatalf("clear fingerprint: %v", err)
	}
	view, err := svc.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("get legacy run: %v", err)
	}
	if !view.NetworkFresh {
		t.Fatal("legacy runs without fingerprint must be treated as fresh to preserve original flow")
	}
}
