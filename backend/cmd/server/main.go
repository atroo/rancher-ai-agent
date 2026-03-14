package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/atroo/rancher-ai-assistant/backend/internal/agent"
	anthropicprovider "github.com/atroo/rancher-ai-assistant/backend/internal/agent/anthropic"
	"github.com/atroo/rancher-ai-assistant/backend/internal/api"
	"github.com/atroo/rancher-ai-assistant/backend/internal/config"
	"github.com/atroo/rancher-ai-assistant/backend/internal/datasource"
	"github.com/atroo/rancher-ai-assistant/backend/internal/embedding"
	"github.com/atroo/rancher-ai-assistant/backend/internal/storage"
	"github.com/atroo/rancher-ai-assistant/backend/internal/tool"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Open SQLite database for persistent sessions and long-term memory
	dbPath := envOrDefault("DATA_DIR", "/data")
	if err := os.MkdirAll(dbPath, 0o755); err != nil {
		slog.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}
	db, err := storage.Open(filepath.Join(dbPath, "assistant.db"))
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database initialized", "path", dbPath)

	// Initialize data sources based on configured providers
	sources, err := datasource.NewSources(cfg)
	if err != nil {
		slog.Error("failed to initialize datasources", "error", err)
		os.Exit(1)
	}
	slog.Info("datasources initialized",
		"logs", cfg.Logs.Provider,
		"metrics", cfg.Metrics.Provider,
		"traces", cfg.Traces.Provider,
	)

	// Create the LLM provider (Anthropic) with retry
	provider := anthropicprovider.New(cfg.LLMAPIKey, cfg.LLMModel)
	retryProvider := agent.WithRetry(provider, 3, cfg.RetryBaseDelay())

	// Initialize embedding provider (optional — semantic search degrades to text search without it)
	var embedder embedding.Provider
	if cfg.EmbeddingAPIKey != "" {
		embedder = embedding.NewOpenAICompatProvider(embedding.OpenAICompatConfig{
			BaseURL:    cfg.EmbeddingBaseURL,
			APIKey:     cfg.EmbeddingAPIKey,
			Model:      cfg.EmbeddingModel,
			Dimensions: cfg.EmbeddingDimensions,
		})
		slog.Info("embedding provider initialized", "model", cfg.EmbeddingModel, "dimensions", cfg.EmbeddingDimensions)
	} else {
		slog.Info("no EMBEDDING_API_KEY set, semantic search disabled (text search fallback)")
	}

	// Session store backed by SQLite
	sessionStore := agent.NewSQLiteSessionStore(db, cfg.SessionTTL())

	// Build prompt function with memory injection
	promptFn := agent.BuildSystemPromptFn(agent.PromptDeps{DB: db})

	// Create tool registry — agent factory is set after agent creation (circular dep)
	var agentInstance *agent.Agent
	deps := tool.RegistryDeps{
		Sources:  sources,
		DB:       db,
		Embedder: embedder,
		AgentFactory: func() *agent.Agent {
			// Sub-agents get the same provider and tools but a fresh session store
			// to avoid polluting the parent session.
			subRegistry := tool.DefaultRegistry(tool.RegistryDeps{
				Sources:      sources,
				DB:           db,
				Embedder:     embedder,
				AgentFactory: nil, // no nested sub-agents
			})
			return agent.New(retryProvider, subRegistry,
				agent.WithConfig(agent.Config{
					MaxToolRounds:    5, // tighter limit for sub-agents
					MaxTokensPerTurn: 4096,
					MaxTokensBudget:  0,
					ToolConcurrency:  4,
					SessionTTL:       cfg.SessionTTL(),
				}),
				agent.WithMiddleware(agent.LoggingMiddleware),
				agent.WithSystemPrompt(promptFn),
			)
		},
	}
	registry := tool.DefaultRegistry(deps)
	slog.Info("tool registry initialized", "tools", registry.Count())

	// Create the agent
	agentInstance = agent.New(retryProvider, registry,
		agent.WithConfig(agent.Config{
			MaxToolRounds:    10,
			MaxTokensPerTurn: 4096,
			MaxTokensBudget:  0,
			ToolConcurrency:  4,
			SessionTTL:       cfg.SessionTTL(),
		}),
		agent.WithSessionStore(sessionStore),
		agent.WithMiddleware(agent.LoggingMiddleware),
		agent.WithSystemPrompt(promptFn),
	)
	_ = agentInstance // used via handler

	// Set up HTTP server
	mux := http.NewServeMux()
	handler := api.NewHandler(agentInstance)
	handler.RegisterRoutes(mux)

	memoryHandler := api.NewMemoryHandler(db)
	memoryHandler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: api.AuthMiddleware(mux),
	}

	slog.Info("starting server", "addr", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
