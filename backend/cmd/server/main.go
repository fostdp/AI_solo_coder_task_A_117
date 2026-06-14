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

	"waterwheel-monitor/internal/alarm_mqtt"
	"waterwheel-monitor/internal/config"
	"waterwheel-monitor/internal/database"
	"waterwheel-monitor/internal/dtu_receiver"
	"waterwheel-monitor/internal/handlers"
	"waterwheel-monitor/internal/hydraulic_model"
	"waterwheel-monitor/internal/models"
	"waterwheel-monitor/internal/pipeline"
	"waterwheel-monitor/internal/shape_optimizer"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	hydraulicParams := config.DefaultHydraulicParams()
	optimizerParams := config.DefaultOptimizerParams()
	alarmParams := config.DefaultAlarmParams()
	receiverParams := config.DefaultReceiverParams()

	db, err := database.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected successfully")

	chans := pipeline.NewChannels(1024)
	defer chans.Close()

	mqttCfg := &models.MQTTConfig{
		BrokerURL: cfg.MQTTBroker,
		ClientID:  cfg.MQTTClientID,
		Username:  cfg.MQTTUsername,
		Password:  cfg.MQTTPassword,
		TopicPrefix: cfg.MQTTTopicPrefix,
	}

	receiver := dtu_receiver.New(db, chans, receiverParams)
	hydraulic := hydraulic_model.New(db, chans, hydraulicParams)
	optimizer := shape_optimizer.New(db, chans, optimizerParams, hydraulicParams)
	alerter := alarm_mqtt.New(db, chans, mqttCfg, alarmParams)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	receiver.Start(rootCtx)
	hydraulic.Start(rootCtx)
	optimizer.Start(rootCtx)
	alerter.Start(rootCtx)

	h := handlers.NewV2(db, chans, hydraulic, optimizer, cfg.EfficiencyAlertThreshold)

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
		api.GET("/health", receiver.HandleHealth)

		api.GET("/waterwheels", h.GetWaterwheels)
		api.GET("/waterwheels/:id", h.GetWaterwheel)

		api.POST("/telemetry", receiver.HandleReportTelemetry)
		api.GET("/waterwheels/:id/telemetry", h.GetTelemetry)
		api.GET("/waterwheels/:id/telemetry/range", h.GetTelemetryRange)

		api.GET("/waterwheels/:id/efficiency", h.GetEfficiencyAnalysis)

		api.GET("/waterwheels/:id/alerts", h.GetAlerts)

		api.POST("/waterwheels/:id/optimize", h.RunOptimizationV2)
		api.GET("/waterwheels/:id/optimizations", h.GetOptimizationResults)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on port %s | Hydraulic workers:%d Optimizer workers:%d",
			cfg.ServerPort, 2, 1)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down modules...")

	rootCancel()

	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly")
}
