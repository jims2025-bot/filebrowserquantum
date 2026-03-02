package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/fileutils"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/people"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
	"github.com/jims2025-bot/filebrowserquantum/backend/facerec"
	"github.com/jims2025-bot/filebrowserquantum/backend/heatmap"
	fbhttp "github.com/jims2025-bot/filebrowserquantum/backend/http"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/integrity"
	"github.com/jims2025-bot/filebrowserquantum/backend/iptcindex"
	"github.com/jims2025-bot/filebrowserquantum/backend/jobs"
	"github.com/jims2025-bot/filebrowserquantum/backend/preview"
	"github.com/jims2025-bot/filebrowserquantum/backend/swagger/docs"
	"github.com/swaggo/swag"

	"github.com/jims2025-bot/filebrowserquantum/backend/common/version"
)

var store *storage.Storage

func getStore(configFile string) bool {
	// Use the config file (global flag)
	settings.Initialize(configFile)
	s, hasDB, err := storage.InitializeDb(settings.Config.Server.Database)
	if err != nil {
		logger.Fatalf("could not load db info: %v", err)
	}
	store = s
	return hasDB
}

func generalUsage() {
	fmt.Printf(`usage: ./filebrowser <command> [options]
commands:
	-h    	Print help
	-c    	Print the default config file
	version Print version information
	set -u	Username and password for the new user
	set -a	Create user as admin
	set -s	Specify a user scope
	set -h	Print this help message
`)
}

func StartFilebrowser() {
	keepGoing := runCLI()
	if !keepGoing {
		return
	}
	// Create context and channels for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})             // Signals server has stopped
	shutdownComplete := make(chan struct{}) // Signals shutdown process is complete
	dbExists := getStore(configPath)
	database := fmt.Sprintf("Using existing database  : %v", settings.Config.Server.Database)
	if !dbExists {
		database = fmt.Sprintf("Creating new database    : %v", settings.Config.Server.Database)
	}
	sourceList := []string{}
	for path, source := range settings.Config.Server.SourceMap {
		sourceList = append(sourceList, fmt.Sprintf("%v: %v", source.Name, path))
	}
	logger.Infof("Initializing FileBrowser Quantum (%v)", version.Version)
	logger.Infof("Using Config file        : %v", configPath)
	logger.Infof("Auth Methods             : %v", settings.Config.Auth.AuthMethods)
	logger.Info(database)
	logger.Infof("Sources                  : %v", sourceList)

	// Initialize facial recognition database
	people.InitDB(settings.Config.Server.Database + "_people")

	serverConfig := settings.Config.Server
	swagInfo := docs.SwaggerInfo
	swagInfo.BasePath = serverConfig.BaseURL
	swag.Register(docs.SwaggerInfo.InstanceName(), swagInfo)
	// initialize indexing and schedule indexing ever n minutes (default 5)
	if len(settings.Config.Server.SourceMap) == 0 {
		logger.Fatal("No sources configured, exiting...")
	}
	for _, source := range settings.Config.Server.SourceMap {
		go indexing.Initialize(source, false)
	}
	validateUserInfo()
	validateOfficeIntegration()

	// Register weekly scheduled jobs — no startup execution, no chaining.
	// Heatmap: Monday 3:01 AM
	jobs.Register(
		"heatmap",
		"GPS heatmap generation — scans images for coordinates and builds cluster data",
		time.Monday, 3, 1,
		func() { heatmap.ScanAllSources(store) },
	)
	// Integrity Scan: Tuesday 3:01 AM
	jobs.Register(
		"integrity",
		"File integrity scan — checks images for corruption, truncation, and format errors",
		time.Tuesday, 3, 1,
		func() { integrity.RunScan(store) },
	)
	// IPTC Index: Wednesday 3:01 AM
	jobs.Register(
		"iptcindex",
		"IPTC metadata index — catalogues notes, captions, and photo dates for every image",
		time.Wednesday, 3, 1,
		func() { iptcindex.ScanAllSources(store) },
	)
	// Facial Recognition: Thursday 3:01 AM
	jobs.Register(
		"facescan",
		"Facial Recognition scanner — analyzes images with ML and integrates ACDSee face regions",
		time.Thursday, 3, 1,
		func() { facerec.ScanAllSources(&settings.Config, store) },
	)
	jobs.StartAll()

	// Start Map Overlay Job (keeps its own frequency-based schedule)
	heatmap.StartOverlayJob(store)

	// Start User Expiration Job
	storage.StartExpirationJob()

	// Start the rootCMD in a goroutine
	go func() {
		if err := rootCMD(ctx, store, &serverConfig, shutdownComplete); err != nil {
			logger.Fatalf("Error starting filebrowser: %v", err)
		}
		close(done) // Signal that the server has stopped
	}()
	// Wait for a shutdown signal or the server to stop
	select {
	case <-signalChan:
		logger.Info("Received shutdown signal. Shutting down gracefully...")
		cancel() // Trigger context cancellation
	case <-done:
		logger.Info("Server stopped unexpectedly. Shutting down...")
	}
	fileutils.ClearCacheDir()

	<-shutdownComplete // Ensure we don't exit prematurely
	// Wait for the server to stop
	logger.Info("Shutdown complete.")
}

func rootCMD(ctx context.Context, store *storage.Storage, serverConfig *settings.Server, shutdownComplete chan struct{}) error {
	if serverConfig.NumImageProcessors < 1 {
		logger.Fatal("Image resize workers count could not be < 1")
	}
	cacheDir := settings.Config.Server.CacheDir
	numWorkers := settings.Config.Server.NumImageProcessors
	ffpmpegPath := settings.Config.Integrations.Media.FfmpegPath
	// setup disk cache
	err := preview.StartPreviewGenerator(numWorkers, ffpmpegPath, cacheDir)
	if err != nil {
		logger.Fatalf("Error starting preview service: %v", err)
	}
	fbhttp.StartHttp(ctx, store, shutdownComplete)
	return nil
}
