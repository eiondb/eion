package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// this is a pointer so that if someone attempts to use it before loading it will
// panic and force them to load it first.
// it is also private so that it cannot be modified after loading.
var _loaded *Config

// Config is the main configuration structure
type Config struct {
	Common Common `yaml:"common"`
}

// Load loads the configuration following proper precedence: defaults → config file → environment variables
func Load() {
	// Start with defaults
	_loaded = &defaultConfig

	// Try to load from config file and merge over defaults
	configFile := os.Getenv("EION_CONFIG_FILE")
	if configFile == "" {
		configFile = "eion.yaml"
	}

	log.Printf("Attempting to load config file: %s", configFile)

	if err := LoadFromFile(configFile); err != nil {
		log.Printf("Failed to load config file: %v, using defaults", err)
	} else {
		log.Printf("Successfully loaded config from file: %s", configFile)
	}

	// Apply environment variable overrides (highest priority)
	ApplyEnvOverrides()

	// Debug log the final config
	if _loaded != nil {
		log.Printf("Final config - DB Host: %s, DB User: %s, DB Database: %s",
			_loaded.Common.Postgres.Host,
			_loaded.Common.Postgres.User,
			_loaded.Common.Postgres.Database)
	}
}

func LoadDefault() {
	config := defaultConfig
	_loaded = &config
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Start with defaults
	cfg := defaultConfig

	// Merge YAML values over defaults
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	_loaded = &cfg
	log.Printf("Loaded config from file - DB Host: %s, DB User: %s", cfg.Common.Postgres.Host, cfg.Common.Postgres.User)
	return nil
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() {
	config := defaultConfig

	// Override with environment variables if present
	if dbHost := os.Getenv("EION_DB_HOST"); dbHost != "" {
		config.Common.Postgres.Host = dbHost
	}
	if dbPort := os.Getenv("EION_DB_PORT"); dbPort != "" {
		if port, err := strconv.Atoi(dbPort); err == nil {
			config.Common.Postgres.Port = port
		}
	}
	if dbUser := os.Getenv("EION_DB_USER"); dbUser != "" {
		config.Common.Postgres.User = dbUser
	}
	if dbPassword := os.Getenv("EION_DB_PASSWORD"); dbPassword != "" {
		config.Common.Postgres.Password = dbPassword
	}
	if dbName := os.Getenv("EION_DB_NAME"); dbName != "" {
		config.Common.Postgres.Database = dbName
	}

	if redisHost := os.Getenv("EION_REDIS_HOST"); redisHost != "" {
		config.Common.Redis.Host = redisHost
	}
	if redisPort := os.Getenv("EION_REDIS_PORT"); redisPort != "" {
		if port, err := strconv.Atoi(redisPort); err == nil {
			config.Common.Redis.Port = port
		}
	}

	if httpHost := os.Getenv("EION_HTTP_HOST"); httpHost != "" {
		config.Common.Http.Host = httpHost
	}
	if httpPort := os.Getenv("EION_HTTP_PORT"); httpPort != "" {
		if port, err := strconv.Atoi(httpPort); err == nil {
			config.Common.Http.Port = port
		}
	}

	// Numa configuration from environment variables
	if numaEnabled := os.Getenv("EION_NUMA_ENABLED"); numaEnabled != "" {
		if enabled, err := strconv.ParseBool(numaEnabled); err == nil {
			config.Common.Numa.Enabled = enabled // Direct assignment since field is "Enabled"
		}
	}
	if openaiAPIKey := os.Getenv("EION_OPENAI_API_KEY"); openaiAPIKey != "" {
		config.Common.Numa.OpenAIAPIKey = openaiAPIKey
	}
	if embeddingModel := os.Getenv("EION_EMBEDDING_MODEL"); embeddingModel != "" {
		config.Common.Numa.EmbeddingModel = embeddingModel
	}
	if neo4jURI := os.Getenv("EION_NEO4J_URI"); neo4jURI != "" {
		config.Common.Numa.Neo4j.URI = neo4jURI
	}
	if neo4jUsername := os.Getenv("EION_NEO4J_USERNAME"); neo4jUsername != "" {
		config.Common.Numa.Neo4j.Username = neo4jUsername
	}
	if neo4jPassword := os.Getenv("EION_NEO4J_PASSWORD"); neo4jPassword != "" {
		config.Common.Numa.Neo4j.Password = neo4jPassword
	}
	if neo4jDatabase := os.Getenv("EION_NEO4J_DATABASE"); neo4jDatabase != "" {
		config.Common.Numa.Neo4j.Database = neo4jDatabase
	}

	_loaded = &config
}

// set sane defaults for all of the config options. when loading the config from
// the file, any options that are not set will be set to these defaults.
var defaultConfig = Config{
	Common: Common{
		Log: logConfig{
			Level:  "info",
			Format: "json",
		},
		Http: httpConfig{
			Host:           "0.0.0.0",
			Port:           8080,
			MaxRequestSize: 5242880,
		},
		Auth: authConfig{
			ClusterAPIKey: "eion_cluster_default_key", // Default key for development
		},
		Postgres: postgresConfig{
			postgresConfigCommon: postgresConfigCommon{
				User:               "postgres",
				Password:           "postgres",
				Host:               "localhost",
				Port:               5432,
				Database:           "eion",
				SchemaName:         "public",
				ReadTimeout:        30,
				WriteTimeout:       30,
				MaxOpenConnections: 10,
			},
		},
		Redis: redisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			Database: 0,
		},
		Carbon: carbonConfig{
			Locale: "en",
		},
		Memory: memoryConfig{
			EnableExtraction: true, // Enable Knowledge functionality by default in Eion base
			VectorStoreType:  "postgres",
			TokenCounterType: "simple",
		},
		Numa: numaConfig{
			Enabled:        false, // DISABLED by default for Phase 1 Week 1.2 Subphase 1
			OpenAIAPIKey:   "",
			EmbeddingModel: "",
			Neo4j:          neo4jConfig{},
		},
	},
}

type Common struct {
	Log      logConfig      `yaml:"log"`
	Http     httpConfig     `yaml:"http"`
	Auth     authConfig     `yaml:"auth"`
	Postgres postgresConfig `yaml:"postgres"`
	Redis    redisConfig    `yaml:"redis"`
	Carbon   carbonConfig   `yaml:"carbon"`
	Memory   memoryConfig   `yaml:"memory"`
	Numa     numaConfig     `yaml:"numa"`
}

type logConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type httpConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	MaxRequestSize int64  `yaml:"max_request_size"`
}

type postgresConfigCommon struct {
	User               string `yaml:"user"`
	Password           string `yaml:"password"`
	Host               string `yaml:"host"`
	Port               int    `yaml:"port"`
	Database           string `yaml:"database"`
	SchemaName         string `yaml:"schema_name"`
	ReadTimeout        int    `yaml:"read_timeout"`
	WriteTimeout       int    `yaml:"write_timeout"`
	MaxOpenConnections int    `yaml:"max_open_connections"`
}

func (c postgresConfigCommon) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(c.User),
		url.QueryEscape(c.Password),
		c.Host,
		c.Port,
		url.QueryEscape(c.Database),
	)
}

type postgresConfig struct {
	postgresConfigCommon `yaml:",inline"`
}

type redisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	Database int    `yaml:"database"`
}

func (c redisConfig) DSN() string {
	if c.Password != "" {
		return fmt.Sprintf("redis://:%s@%s:%d/%d", c.Password, c.Host, c.Port, c.Database)
	}
	return fmt.Sprintf("redis://%s:%d/%d", c.Host, c.Port, c.Database)
}

type carbonConfig struct {
	// should be the name of one of the language files in carbon
	// https://github.com/golang-module/carbon/tree/master/lang
	Locale string `yaml:"locale"`
}

type memoryConfig struct {
	EnableExtraction bool   `yaml:"enable_extraction"`  // Enable Knowledge functionality (knowledge extraction with Numa)
	VectorStoreType  string `yaml:"vector_store_type"`  // "postgres" or "qdrant"
	TokenCounterType string `yaml:"token_counter_type"` // "tiktoken" or "simple"
}

type authConfig struct {
	ClusterAPIKey string `yaml:"cluster_api_key"` // API key for cluster management operations
}

type numaConfig struct {
	Enabled        bool        `yaml:"enabled"`         // Master switch to enable Numa functionality
	OpenAIAPIKey   string      `yaml:"openai_api_key"`  // OpenAI API key for LLM
	EmbeddingModel string      `yaml:"embedding_model"` // Local embedding model name
	Neo4j          neo4jConfig `yaml:"neo4j"`           // Neo4j configuration
}

type neo4jConfig struct {
	URI      string `yaml:"uri"`      // Neo4j connection URI
	Username string `yaml:"username"` // Neo4j username
	Password string `yaml:"password"` // Neo4j password
	Database string `yaml:"database"` // Neo4j database name
}

// there should be a getter for each top level field in the config struct.
// these getters will panic if the config has not been loaded.

func Logger() logConfig {
	if _loaded == nil {
		panic("config not loaded - call Load() first")
	}
	return _loaded.Common.Log
}

func Http() httpConfig {
	if _loaded == nil {
		panic("config not loaded - call Load() first")
	}
	return _loaded.Common.Http
}

func Auth() authConfig {
	if _loaded == nil {
		panic("config not loaded - call Load() first")
	}
	return _loaded.Common.Auth
}

func Postgres() postgresConfig {
	if _loaded == nil {
		panic("config not loaded - call Load() first")
	}
	return _loaded.Common.Postgres
}

func Redis() redisConfig {
	if _loaded == nil {
		panic("config not loaded - call Load() first")
	}
	return _loaded.Common.Redis
}

func Carbon() carbonConfig {
	if _loaded == nil {
		panic("config not loaded - call Load() first")
	}
	return _loaded.Common.Carbon
}

func Memory() memoryConfig {
	if _loaded == nil {
		panic("config not loaded - call Load() first")
	}
	return _loaded.Common.Memory
}

// Get returns the full configuration
func Numa() numaConfig {
	if _loaded == nil {
		panic("config not loaded")
	}
	return _loaded.Common.Numa
}

func Get() *Config {
	if _loaded == nil {
		panic("config not loaded - call Load() first")
	}
	return _loaded
}

func ApplyEnvOverrides() {
	if _loaded == nil {
		return
	}

	// Override with environment variables if present
	if dbHost := os.Getenv("EION_DB_HOST"); dbHost != "" {
		_loaded.Common.Postgres.Host = dbHost
	}
	if dbPort := os.Getenv("EION_DB_PORT"); dbPort != "" {
		if port, err := strconv.Atoi(dbPort); err == nil {
			_loaded.Common.Postgres.Port = port
		}
	}
	if dbUser := os.Getenv("EION_DB_USER"); dbUser != "" {
		_loaded.Common.Postgres.User = dbUser
	}
	if dbPassword := os.Getenv("EION_DB_PASSWORD"); dbPassword != "" {
		_loaded.Common.Postgres.Password = dbPassword
	}
	if dbName := os.Getenv("EION_DB_NAME"); dbName != "" {
		_loaded.Common.Postgres.Database = dbName
	}

	if redisHost := os.Getenv("EION_REDIS_HOST"); redisHost != "" {
		_loaded.Common.Redis.Host = redisHost
	}
	if redisPort := os.Getenv("EION_REDIS_PORT"); redisPort != "" {
		if port, err := strconv.Atoi(redisPort); err == nil {
			_loaded.Common.Redis.Port = port
		}
	}

	if httpHost := os.Getenv("EION_HTTP_HOST"); httpHost != "" {
		_loaded.Common.Http.Host = httpHost
	}
	if httpPort := os.Getenv("EION_HTTP_PORT"); httpPort != "" {
		if port, err := strconv.Atoi(httpPort); err == nil {
			_loaded.Common.Http.Port = port
		}
	}

	// Auth configuration from environment variables
	if clusterAPIKey := os.Getenv("EION_CLUSTER_API_KEY"); clusterAPIKey != "" {
		_loaded.Common.Auth.ClusterAPIKey = clusterAPIKey
	}

	// Numa configuration from environment variables
	if numaEnabled := os.Getenv("EION_NUMA_ENABLED"); numaEnabled != "" {
		if enabled, err := strconv.ParseBool(numaEnabled); err == nil {
			_loaded.Common.Numa.Enabled = enabled
		}
	}
	if openaiAPIKey := os.Getenv("EION_OPENAI_API_KEY"); openaiAPIKey != "" {
		_loaded.Common.Numa.OpenAIAPIKey = openaiAPIKey
	}
	if embeddingModel := os.Getenv("EION_EMBEDDING_MODEL"); embeddingModel != "" {
		_loaded.Common.Numa.EmbeddingModel = embeddingModel
	}
	if neo4jURI := os.Getenv("EION_NEO4J_URI"); neo4jURI != "" {
		_loaded.Common.Numa.Neo4j.URI = neo4jURI
	}
	if neo4jUsername := os.Getenv("EION_NEO4J_USERNAME"); neo4jUsername != "" {
		_loaded.Common.Numa.Neo4j.Username = neo4jUsername
	}
	if neo4jPassword := os.Getenv("EION_NEO4J_PASSWORD"); neo4jPassword != "" {
		_loaded.Common.Numa.Neo4j.Password = neo4jPassword
	}
	if neo4jDatabase := os.Getenv("EION_NEO4J_DATABASE"); neo4jDatabase != "" {
		_loaded.Common.Numa.Neo4j.Database = neo4jDatabase
	}
}
