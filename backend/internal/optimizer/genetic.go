package optimizer

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"waterwheel-monitor/internal/models"
)

type Individual struct {
	Params  models.BucketParams
	Fitness float64
}

type GAOptimizer struct {
	populationSize int
	generations    int
	mutationRate   float64
	crossoverRate  float64
	rand           *rand.Rand
}

func NewGAOptimizer() *GAOptimizer {
	return &GAOptimizer{
		populationSize: 100,
		generations:    200,
		mutationRate:   0.15,
		crossoverRate:  0.8,
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (ga *GAOptimizer) Optimize(wheel *models.Waterwheel, currentData *models.TelemetryData) *models.OptimizationResult {
	baseline := ga.evaluateFitness(wheel, currentData, models.BucketParams{
		Width:     wheel.BucketCapacity * 10,
		Depth:     wheel.BucketCapacity * 5,
		Height:    wheel.BucketCapacity * 8,
		Angle:     15.0,
		Curvature: 0.3,
	})

	population := ga.initializePopulation()
	fitnessHistory := make([]float64, 0, ga.generations)

	best := population[0]
	best.Fitness = ga.evaluateFitness(wheel, currentData, best.Params)

	for gen := 0; gen < ga.generations; gen++ {
		for i := range population {
			population[i].Fitness = ga.evaluateFitness(wheel, currentData, population[i].Params)
		}

		sort.Slice(population, func(i, j int) bool {
			return population[i].Fitness > population[j].Fitness
		})

		if population[0].Fitness > best.Fitness {
			best = population[0]
		}
		fitnessHistory = append(fitnessHistory, best.Fitness)

		if gen < ga.generations-1 {
			population = ga.nextGeneration(population)
		}
	}

	improvement := 0.0
	if baseline > 0 {
		improvement = (best.Fitness - baseline) / baseline * 100.0
	}

	return &models.OptimizationResult{
		WaterwheelID: wheel.ID,
		BucketShapeParams: map[string]float64{
			"width":     best.Params.Width,
			"depth":     best.Params.Depth,
			"height":    best.Params.Height,
			"curvature": best.Params.Curvature,
		},
		BucketAngle:        best.Params.Angle,
		OptimizedLiftRate:  best.Fitness,
		OriginalLiftRate:   baseline,
		ImprovementPercent: improvement,
		GenerationCount:    ga.generations,
		FitnessHistory:     fitnessHistory,
	}
}

func (ga *GAOptimizer) initializePopulation() []Individual {
	pop := make([]Individual, ga.populationSize)
	for i := range pop {
		pop[i] = Individual{
			Params: models.BucketParams{
				Width:     0.1 + ga.rand.Float64()*0.9,
				Depth:     0.05 + ga.rand.Float64()*0.5,
				Height:    0.1 + ga.rand.Float64()*0.8,
				Angle:     ga.rand.Float64()*45.0 - 5.0,
				Curvature: ga.rand.Float64()*0.8 + 0.1,
			},
		}
	}
	return pop
}

func (ga *GAOptimizer) evaluateFitness(wheel *models.Waterwheel, data *models.TelemetryData, params models.BucketParams) float64 {
	bucketVolume := params.Width * params.Depth * params.Height * params.Curvature * 0.7
	if bucketVolume <= 0 {
		return 0
	}

	angleRad := params.Angle * math.Pi / 180.0
	fillEfficiency := 0.3 + 0.6*math.Sin(angleRad+math.Pi/6)
	fillEfficiency = math.Max(0.1, math.Min(0.95, fillEfficiency))

	effectiveCount := float64(wheel.BucketCount) * fillEfficiency
	volumePerRotation := effectiveCount * bucketVolume

	theoreticalLift := volumePerRotation * data.RotationSpeed * 60.0
	theoreticalLift = math.Min(theoreticalLift, wheel.MaxFlowRate*1.2)

	radius := wheel.Diameter / 2.0
	waterWeight := WaterDensity * Gravity * bucketVolume * fillEfficiency
	torquePerBucket := waterWeight * radius * math.Cos(angleRad)

	dragCoeff := 0.5 + params.Curvature*0.5
	dragLoss := dragCoeff * data.FlowVelocity * data.FlowVelocity * params.Width * params.Height * 0.5

	omega := data.RotationSpeed * 2 * math.Pi / 60.0
	centrifugalLoss := WaterDensity * bucketVolume * omega * omega * radius * 0.005

	totalTorque := float64(wheel.BucketCount/4)*torquePerBucket - dragLoss - centrifugalLoss
	totalTorque = math.Max(0, totalTorque)

	powerOutput := totalTorque * omega

	liftHeight := wheel.Diameter * 0.9
	actualLift := 0.0
	if liftHeight > 0 && Gravity > 0 {
		actualLift = powerOutput / (WaterDensity * Gravity * liftHeight) * 3600.0
	}

	fitness := actualLift * 0.7 + theoreticalLift * 0.3
	return math.Max(0, fitness)
}

func (ga *GAOptimizer) nextGeneration(pop []Individual) []Individual {
	next := make([]Individual, 0, ga.populationSize)

	elitism := 5
	for i := 0; i < elitism && i < len(pop); i++ {
		next = append(next, pop[i])
	}

	for len(next) < ga.populationSize {
		parent1 := ga.tournamentSelect(pop)
		parent2 := ga.tournamentSelect(pop)

		var child Individual
		if ga.rand.Float64() < ga.crossoverRate {
			child = ga.crossover(parent1, parent2)
		} else {
			child = parent1
		}

		if ga.rand.Float64() < ga.mutationRate {
			child = ga.mutate(child)
		}

		next = append(next, child)
	}

	return next
}

func (ga *GAOptimizer) tournamentSelect(pop []Individual) Individual {
	tournamentSize := 5
	bestIdx := ga.rand.Intn(len(pop))
	for i := 1; i < tournamentSize; i++ {
		idx := ga.rand.Intn(len(pop))
		if pop[idx].Fitness > pop[bestIdx].Fitness {
			bestIdx = idx
		}
	}
	return pop[bestIdx]
}

func (ga *GAOptimizer) crossover(p1, p2 Individual) Individual {
	return Individual{
		Params: models.BucketParams{
			Width:     ga.blendCrossover(p1.Params.Width, p2.Params.Width),
			Depth:     ga.blendCrossover(p1.Params.Depth, p2.Params.Depth),
			Height:    ga.blendCrossover(p1.Params.Height, p2.Params.Height),
			Angle:     ga.blendCrossover(p1.Params.Angle, p2.Params.Angle),
			Curvature: ga.blendCrossover(p1.Params.Curvature, p2.Params.Curvature),
		},
	}
}

func (ga *GAOptimizer) blendCrossover(a, b float64) float64 {
	alpha := 0.5
	min := math.Min(a, b)
	max := math.Max(a, b)
	rangeVal := max - min
	return min - alpha*rangeVal + ga.rand.Float64()*(rangeVal*(1+2*alpha))
}

func (ga *GAOptimizer) mutate(ind Individual) Individual {
	mutateGene := func(val, min, max, sigma float64) float64 {
		if ga.rand.Float64() < 0.5 {
			val += ga.rand.NormFloat64() * sigma
		}
		return math.Max(min, math.Min(max, val))
	}

	ind.Params.Width = mutateGene(ind.Params.Width, 0.05, 2.0, 0.1)
	ind.Params.Depth = mutateGene(ind.Params.Depth, 0.02, 1.0, 0.05)
	ind.Params.Height = mutateGene(ind.Params.Height, 0.05, 1.5, 0.08)
	ind.Params.Angle = mutateGene(ind.Params.Angle, -10.0, 50.0, 3.0)
	ind.Params.Curvature = mutateGene(ind.Params.Curvature, 0.1, 1.0, 0.1)

	return ind
}

const (
	WaterDensity = 1000.0
	Gravity      = 9.81
)
