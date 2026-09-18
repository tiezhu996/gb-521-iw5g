package model

import (
	"time"

	"gorm.io/datatypes"
)

type SimulationRun struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	ScenarioID        uint           `gorm:"not null;index" json:"scenario_id"`
	RunStatus         string         `gorm:"size:24;not null;index;check:run_status IN ('queued','running','converged','not_converged','invalid_input','failed')" json:"run_status"`
	IterationCount    int            `gorm:"not null;default:0" json:"iteration_count"`
	Residual          float64        `gorm:"not null;default:0" json:"residual"`
	InputSnapshotJSON datatypes.JSON `gorm:"type:jsonb;not null" json:"input_snapshot_json"`
	NodePressuresJSON datatypes.JSON `gorm:"type:jsonb;not null" json:"node_pressures_json"`
	EdgeFlowsJSON     datatypes.JSON `gorm:"type:jsonb;not null" json:"edge_flows_json"`
	ResidualsJSON     datatypes.JSON `gorm:"type:jsonb;not null" json:"residuals_json"`
	RiskFlagsJSON     datatypes.JSON `gorm:"type:jsonb;not null" json:"risk_flags_json"`
	NetworkFingerprintJSON datatypes.JSON `gorm:"type:jsonb" json:"network_fingerprint_json"`
	AlgorithmVersion  string         `gorm:"size:32;not null" json:"algorithm_version"`
	StartedBy         uint           `gorm:"not null;index" json:"started_by"`
	StartedAt         time.Time      `json:"started_at"`
	FinishedAt        *time.Time     `json:"finished_at"`
	RiskConfirmedBy   *uint          `gorm:"index" json:"risk_confirmed_by"`
	RiskConfirmedAt   *time.Time     `json:"risk_confirmed_at"`
	ConfirmationNote  string         `gorm:"size:500" json:"confirmation_note"`
	Scenario          FanScenario    `gorm:"foreignKey:ScenarioID" json:"scenario,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

func (SimulationRun) TableName() string { return "simulation_runs" }

// NetworkFingerprint 是发起推演时刻“已启用节点 + 已启用巷道”关键参数的指纹，
// 用于事后判断历史推演所依据的网络拓扑/参数是否仍然有效。
type NetworkFingerprint struct {
	Version string                  `json:"version"`
	Nodes   []NodeFingerprint       `json:"nodes"`
	Edges   []EdgeFingerprint       `json:"edges"`
}

type NodeFingerprint struct {
	ID                 uint    `json:"id"`
	Code               string  `json:"code"`
	NodeType           string  `json:"node_type"`
	ElevationM         float64 `json:"elevation_m"`
	RequiredAirflowM3S float64 `json:"required_airflow_m3s"`
	PressurePa         float64 `json:"pressure_pa"`
	Status             string  `json:"status"`
}

type EdgeFingerprint struct {
	ID              uint    `json:"id"`
	Code            string  `json:"code"`
	FromNodeID      uint    `json:"from_node_id"`
	ToNodeID        uint    `json:"to_node_id"`
	ResistanceNS2M8 float64 `json:"resistance_ns2m8"`
	AreaM2          float64 `json:"area_m2"`
	MaxVelocityMS   float64 `json:"max_velocity_ms"`
	DoorState       string  `json:"door_state"`
	Enabled         bool    `json:"enabled"`
	CriticalPath    bool    `json:"critical_path"`
}

// NetworkChange 描述指纹与当前网络之间的一处差异。
type NetworkChange struct {
	EntityType string            `json:"entity_type"` // ventilation_node | airway_edge
	EntityID   uint              `json:"entity_id"`
	Code       string            `json:"code"`
	ChangeType string            `json:"change_type"` // modified | added | removed
	Fields     []string          `json:"fields,omitempty"`
	Before     map[string]any    `json:"before,omitempty"`
	After      map[string]any    `json:"after,omitempty"`
}

// SimulationRunView 在持久化记录之外附带网络新鲜度判定结果，
// 历史结果与人工确认仍以 SimulationRun 原值原样返回。
type SimulationRunView struct {
	*SimulationRun
	NetworkFresh     bool            `json:"network_fresh"`
	NetworkChanges   []NetworkChange `json:"network_changes"`
}
