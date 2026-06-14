package efficiency

import (
	"math"
	"time"

	"waterwheel-monitor/internal/models"
)

const (
	WaterDensity     = 1000.0
	Gravity          = 9.81
	FrictionCoeff    = 0.08
	BearingFriction  = 0.05
)

type Calculator struct{}

func NewCalculator() *Calculator {
	return &Calculator{}
}

func (c *Calculator) Analyze(wheel *models.Waterwheel, data *models.TelemetryData) *models.EfficiencyAnalysis {
	radius := wheel.Diameter / 2.0
	angularVelocity := data.RotationSpeed * 2 * math.Pi / 60.0

	torqueInput := c.calculateHydraulicTorque(wheel, data, radius)
	torqueOutput := c.calculateOutputTorque(wheel, data, radius, angularVelocity)
	liftResistance := c.calculateLiftResistance(wheel, data, radius)

	netTorque := torqueInput - torqueOutput - liftResistance -
		BearingFriction*torqueInput - FrictionCoeff*math.Abs(angularVelocity)

	inputPower := torqueInput * angularVelocity
	outputPower := math.Max(0, netTorque) * angularVelocity

	mechEff := 0.0
	if inputPower > 0 {
		mechEff = math.Max(0, math.Min(1, outputPower/inputPower))
	}

	theoreticalLift := c.calculateTheoreticalLift(wheel, data)
	hydEff := 0.0
	if theoreticalLift > 0 {
		hydEff = math.Max(0, math.Min(1, data.WaterLift/theoreticalLift))
	}

	return &models.EfficiencyAnalysis{
		WaterwheelID:         data.WaterwheelID,
		Time:                 data.Time,
		RotationSpeed:        data.RotationSpeed,
		InputPower:           inputPower,
		OutputPower:          outputPower,
		TorqueInput:          torqueInput,
		TorqueOutput:         torqueOutput,
		LiftResistance:       liftResistance,
		MechanicalEfficiency: mechEff,
		HydraulicEfficiency:  hydEff,
		OverallEfficiency:    mechEff * hydEff,
	}
}

func (c *Calculator) calculateHydraulicTorque(wheel *models.Waterwheel, data *models.TelemetryData, radius float64) float64 {
	submergedBuckets := c.calculateSubmergedBuckets(wheel, data)
	bucketForce := WaterDensity * Gravity * wheel.BucketCapacity * 0.7

	effectiveRadius := radius * 0.85
	impactForce := 0.5 * WaterDensity * data.FlowVelocity * data.FlowVelocity *
		wheel.BucketCapacity * 0.5 / radius

	torque := float64(submergedBuckets)*bucketForce*effectiveRadius +
		float64(wheel.BucketCount/4)*impactForce*radius

	return torque
}

func (c *Calculator) calculateSubmergedBuckets(wheel *models.Waterwheel, data *models.TelemetryData) int {
	submersionRatio := math.Min(1, data.WaterLevelDrop/wheel.Diameter)
	return int(math.Max(1, float64(wheel.BucketCount)*submersionRatio*0.4))
}

func (c *Calculator) calculateOutputTorque(wheel *models.Waterwheel, data *models.TelemetryData, radius, omega float64) float64 {
	liftedMassPerSecond := data.WaterLift / 60.0
	liftHeight := wheel.Diameter * 0.9
	potentialPower := liftedMassPerSecond * Gravity * liftHeight

	if omega > 0 {
		return potentialPower / omega
	}
	return 0
}

func (c *Calculator) calculateLiftResistance(wheel *models.Waterwheel, data *models.TelemetryData, radius float64) float64 {
	liftedVolume := data.WaterLift / 60.0 / data.RotationSpeed
	if data.RotationSpeed <= 0 {
		liftedVolume = data.WaterLift / 60.0 / 0.5
	}

	eccentricTorque := WaterDensity * Gravity * liftedVolume * radius * 0.3
	centrifugalLoss := WaterDensity * liftedVolume * data.RotationSpeed * data.RotationSpeed * radius * 0.01

	return eccentricTorque + centrifugalLoss
}

func (c *Calculator) calculateTheoreticalLift(wheel *models.Waterwheel, data *models.TelemetryData) float64 {
	filledBuckets := float64(wheel.BucketCount) * 0.35
	volumePerRotation := filledBuckets * wheel.BucketCapacity
	liftPerMinute := volumePerRotation * data.RotationSpeed * 60.0
	return math.Min(liftPerMinute, wheel.MaxFlowRate)
}

func (c *Calculator) EnrichTelemetry(wheel *models.Waterwheel, data *models.TelemetryData) {
	analysis := c.Analyze(wheel, data)
	mech := analysis.MechanicalEfficiency
	hyd := analysis.HydraulicEfficiency
	torque := analysis.TorqueInput
	power := analysis.OutputPower

	data.MechanicalEfficiency = &mech
	data.HydraulicEfficiency = &hyd
	data.Torque = &torque
	data.PowerOutput = &power

	if data.Time.IsZero() {
		data.Time = time.Now()
	}
}
