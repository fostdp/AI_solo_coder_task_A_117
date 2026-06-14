package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"waterwheel-monitor/internal/config"
	"waterwheel-monitor/internal/database"
	"waterwheel-monitor/internal/efficiency"
	"waterwheel-monitor/internal/handlers"
	"waterwheel-monitor/internal/mqtt"
	"waterwheel-monitor/internal/optimizer"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected successfully")

	effCalc := efficiency.NewCalculator()
	ga := optimizer.NewGAOptimizer()

	var alertClient *mqtt.AlertClient
	alertClient, err = mqtt.NewAlertClient(cfg)
	if err != nil {
		log.Printf("Warning: Failed to connect to MQTT broker (alerts will not be published): %v", err)
		alertClient = nil
	} else {
		defer alertClient.Close()
	}

	h := handlers.New(db, effCalc, ga, alertClient, cfg.EfficiencyAlertThreshold)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Static("/static", "../../frontend")
	r.StaticFile("/", "../../frontend/index.html")

	api := r.Group("/api")
	{
		api.GET("/health", h.HealthCheck)

		api.GET("/waterwheels", h.GetWaterwheels)
		api.GET("/waterwheels/:id", h.GetWaterwheel)

		api.POST("/telemetry", h.ReportTelemetry)
		api.GET("/waterwheels/:id/telemetry", h.GetTelemetry)
		api.GET("/waterwheels/:id/telemetry/range", h.GetTelemetryRange)

		api.GET("/waterwheels/:id/efficiency", h.GetEfficiencyAnalysis)

		api.GET("/waterwheels/:id/alerts", h.GetAlerts)

		api.POST("/waterwheels/:id/optimize", h.RunOptimization)
		api.GET("/waterwheels/:id/optimizations", h.GetOptimizationResults)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
