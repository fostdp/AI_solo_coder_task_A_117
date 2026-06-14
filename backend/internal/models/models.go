package models

import "time"

type Waterwheel struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Location        string    `json:"location"`
	Diameter        float64   `json:"diameter"`
	BucketCount     int       `json:"bucket_count"`
	BucketCapacity  float64   `json:"bucket_capacity"`
	MaxFlowRate     float64   `json:"max_flow_rate"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TelemetryData struct {
	Time                time.Time `json:"time"`
	WaterwheelID        int       `json:"waterwheel_id"`
	RotationSpeed       float64   `json:"rotation_speed"`
	WaterLift           float64   `json:"water_lift"`
	WaterLevelDrop      float64   `json:"water_level_drop"`
	FlowVelocity        float64   `json:"flow_velocity"`
	MechanicalEfficiency *float64  `json:"mechanical_efficiency,omitempty"`
	HydraulicEfficiency  *float64  `json:"hydraulic_efficiency,omitempty"`
	Torque               *float64  `json:"torque,omitempty"`
	PowerOutput          *float64  `json:"power_output,omitempty"`
}

type Alert struct {
	ID              int       `json:"id"`
	WaterwheelID    int       `json:"waterwheel_id"`
	AlertType       string    `json:"alert_type"`
	Message         string    `json:"message"`
	Severity        string    `json:"severity"`
	EfficiencyValue float64   `json:"efficiency_value"`
	HistoricalAvg   float64   `json:"historical_avg"`
	Time            time.Time `json:"time"`
	Acknowledged    bool      `json:"acknowledged"`
}

type OptimizationResult struct {
	ID                 int                    `json:"id"`
	WaterwheelID       int                    `json:"waterwheel_id"`
	BucketShapeParams  map[string]float64     `json:"bucket_shape_params"`
	BucketAngle        float64                `json:"bucket_angle"`
	OptimizedLiftRate  float64                `json:"optimized_lift_rate"`
	OriginalLiftRate   float64                `json:"original_lift_rate"`
	ImprovementPercent float64                `json:"improvement_percent"`
	GenerationCount    int                    `json:"generation_count"`
	FitnessHistory     []float64              `json:"fitness_history,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
}

type EfficiencyAnalysis struct {
	WaterwheelID        int       `json:"waterwheel_id"`
	Time                time.Time `json:"time"`
	RotationSpeed       float64   `json:"rotation_speed"`
	InputPower          float64   `json:"input_power"`
	OutputPower         float64   `json:"output_power"`
	TorqueInput         float64   `json:"torque_input"`
	TorqueOutput        float64   `json:"torque_output"`
	LiftResistance      float64   `json:"lift_resistance"`
	MechanicalEfficiency float64  `json:"mechanical_efficiency"`
	HydraulicEfficiency  float64  `json:"hydraulic_efficiency"`
	OverallEfficiency    float64  `json:"overall_efficiency"`
}

type BucketParams struct {
	Width        float64 `json:"width"`
	Depth        float64 `json:"depth"`
	Height       float64 `json:"height"`
	Angle        float64 `json:"angle"`
	Curvature    float64 `json:"curvature"`
}
