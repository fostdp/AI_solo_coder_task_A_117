package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"waterwheel-monitor/internal/config"
	"waterwheel-monitor/internal/models"
)

type Database struct {
	pool *pgxpool.Pool
}

func New(cfg *config.Config) (*Database, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Database{pool: pool}, nil
}

func (db *Database) Close() {
	db.pool.Close()
}

func (db *Database) GetWaterwheels(ctx context.Context) ([]models.Waterwheel, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, name, location, diameter, bucket_count, bucket_capacity, max_flow_rate, created_at, updated_at
		FROM waterwheels ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wheels []models.Waterwheel
	for rows.Next() {
		var w models.Waterwheel
		if err := rows.Scan(&w.ID, &w.Name, &w.Location, &w.Diameter, &w.BucketCount,
			&w.BucketCapacity, &w.MaxFlowRate, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		wheels = append(wheels, w)
	}
	return wheels, rows.Err()
}

func (db *Database) GetWaterwheelByID(ctx context.Context, id int) (*models.Waterwheel, error) {
	var w models.Waterwheel
	err := db.pool.QueryRow(ctx, `
		SELECT id, name, location, diameter, bucket_count, bucket_capacity, max_flow_rate, created_at, updated_at
		FROM waterwheels WHERE id = $1
	`, id).Scan(&w.ID, &w.Name, &w.Location, &w.Diameter, &w.BucketCount,
		&w.BucketCapacity, &w.MaxFlowRate, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (db *Database) InsertTelemetry(ctx context.Context, data *models.TelemetryData) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO telemetry_data (time, waterwheel_id, rotation_speed, water_lift, water_level_drop,
			flow_velocity, mechanical_efficiency, hydraulic_efficiency, torque, power_output)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, data.Time, data.WaterwheelID, data.RotationSpeed, data.WaterLift, data.WaterLevelDrop,
		data.FlowVelocity, data.MechanicalEfficiency, data.HydraulicEfficiency, data.Torque, data.PowerOutput)
	return err
}

func (db *Database) GetLatestTelemetry(ctx context.Context, waterwheelID int, limit int) ([]models.TelemetryData, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT time, waterwheel_id, rotation_speed, water_lift, water_level_drop,
			flow_velocity, mechanical_efficiency, hydraulic_efficiency, torque, power_output
		FROM telemetry_data WHERE waterwheel_id = $1 ORDER BY time DESC LIMIT $2
	`, waterwheelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []models.TelemetryData
	for rows.Next() {
		var d models.TelemetryData
		if err := rows.Scan(&d.Time, &d.WaterwheelID, &d.RotationSpeed, &d.WaterLift, &d.WaterLevelDrop,
			&d.FlowVelocity, &d.MechanicalEfficiency, &d.HydraulicEfficiency, &d.Torque, &d.PowerOutput); err != nil {
			return nil, err
		}
		data = append(data, d)
	}
	return data, rows.Err()
}

func (db *Database) GetTelemetryRange(ctx context.Context, waterwheelID int, start, end time.Time) ([]models.TelemetryData, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT time, waterwheel_id, rotation_speed, water_lift, water_level_drop,
			flow_velocity, mechanical_efficiency, hydraulic_efficiency, torque, power_output
		FROM telemetry_data WHERE waterwheel_id = $1 AND time BETWEEN $2 AND $3 ORDER BY time ASC
	`, waterwheelID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []models.TelemetryData
	for rows.Next() {
		var d models.TelemetryData
		if err := rows.Scan(&d.Time, &d.WaterwheelID, &d.RotationSpeed, &d.WaterLift, &d.WaterLevelDrop,
			&d.FlowVelocity, &d.MechanicalEfficiency, &d.HydraulicEfficiency, &d.Torque, &d.PowerOutput); err != nil {
			return nil, err
		}
		data = append(data, d)
	}
	return data, rows.Err()
}

func (db *Database) GetHistoricalAvgEfficiency(ctx context.Context, waterwheelID int, hours int) (float64, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	var avg float64
	err := db.pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(COALESCE(mechanical_efficiency, 0) * COALESCE(hydraulic_efficiency, 0)), 0)
		FROM telemetry_data WHERE waterwheel_id = $1 AND time > $2
	`, waterwheelID, since).Scan(&avg)
	return avg, err
}

func (db *Database) InsertAlert(ctx context.Context, alert *models.Alert) error {
	err := db.pool.QueryRow(ctx, `
		INSERT INTO alerts (waterwheel_id, alert_type, message, severity, efficiency_value, historical_avg, time)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id
	`, alert.WaterwheelID, alert.AlertType, alert.Message, alert.Severity,
		alert.EfficiencyValue, alert.HistoricalAvg, alert.Time).Scan(&alert.ID)
	return err
}

func (db *Database) GetAlerts(ctx context.Context, waterwheelID int, limit int) ([]models.Alert, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, waterwheel_id, alert_type, message, severity, efficiency_value, historical_avg, time, acknowledged
		FROM alerts WHERE waterwheel_id = $1 ORDER BY time DESC LIMIT $2
	`, waterwheelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		if err := rows.Scan(&a.ID, &a.WaterwheelID, &a.AlertType, &a.Message, &a.Severity,
			&a.EfficiencyValue, &a.HistoricalAvg, &a.Time, &a.Acknowledged); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (db *Database) InsertOptimizationResult(ctx context.Context, result *models.OptimizationResult) error {
	var fitHist interface{}
	if result.FitnessHistory != nil {
		fitHist = result.FitnessHistory
	}

	err := db.pool.QueryRow(ctx, `
		INSERT INTO optimization_results (waterwheel_id, bucket_shape_params, bucket_angle,
			optimized_lift_rate, original_lift_rate, improvement_percent, generation_count, fitness_history)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, created_at
	`, result.WaterwheelID, result.BucketShapeParams, result.BucketAngle,
		result.OptimizedLiftRate, result.OriginalLiftRate, result.ImprovementPercent,
		result.GenerationCount, fitHist).Scan(&result.ID, &result.CreatedAt)
	return err
}

func (db *Database) GetOptimizationResults(ctx context.Context, waterwheelID int, limit int) ([]models.OptimizationResult, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, waterwheel_id, bucket_shape_params, bucket_angle, optimized_lift_rate,
			original_lift_rate, improvement_percent, generation_count, fitness_history, created_at
		FROM optimization_results WHERE waterwheel_id = $1 ORDER BY created_at DESC LIMIT $2
	`, waterwheelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.OptimizationResult
	for rows.Next() {
		var r models.OptimizationResult
		var params map[string]float64
		var fitHist []float64
		if err := rows.Scan(&r.ID, &r.WaterwheelID, &params, &r.BucketAngle, &r.OptimizedLiftRate,
			&r.OriginalLiftRate, &r.ImprovementPercent, &r.GenerationCount, &fitHist, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.BucketShapeParams = params
		r.FitnessHistory = fitHist
		results = append(results, r)
	}
	return results, rows.Err()
}
