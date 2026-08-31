// Package config handles loading and validation of the ANS Resolution Server configuration.
// It supports declarative YAML configuration with environment variable substitution.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level configuration structure (flat, no spec wrapper)
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Cache      CacheConfig      `yaml:"cache"`
	Queue      QueueConfig      `yaml:"queue"`
	Registries []RegistryConfig `yaml:"registries"`
	Trust      TrustConfig      `yaml:"trust"`
	Policy     PolicyConfig     `yaml:"policy"`
	RateLimit  RateLimitConfig  `yaml:"rateLimit"`
	Telemetry  TelemetryConfig  `yaml:"telemetry"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host                    string        `yaml:"host"`
	Port                    int           `yaml:"port"`
	ReadTimeout             time.Duration `yaml:"readTimeout"`
	WriteTimeout            time.Duration `yaml:"writeTimeout"`
	IdleTimeout             time.Duration `yaml:"idleTimeout"`
	MaxHeaderBytes          int           `yaml:"maxHeaderBytes"`
	GracefulShutdownTimeout time.Duration `yaml:"gracefulShutdownTimeout"`
}

// CacheConfig contains cache provider settings
type CacheConfig struct {
	Provider  string            `yaml:"provider"`
	TTL       CacheTTLConfig    `yaml:"ttl"`
	Memory    MemoryCacheConfig `yaml:"memory"`
	Redis     RedisCacheConfig  `yaml:"redis"`
	Memcached MemcachedConfig   `yaml:"memcached"`
}

// CacheTTLConfig contains TTL settings for different cache states
type CacheTTLConfig struct {
	Default  time.Duration `yaml:"default"`
	Verified time.Duration `yaml:"verified"`
	Revoked  time.Duration `yaml:"revoked"`
	NotFound time.Duration `yaml:"notFound"`
}

// MemoryCacheConfig contains in-memory cache settings
type MemoryCacheConfig struct {
	MaxSize         int           `yaml:"maxSize"`
	CleanupInterval time.Duration `yaml:"cleanupInterval"`
}

// RedisCacheConfig contains Redis cache settings
type RedisCacheConfig struct {
	Address      string         `yaml:"address"`
	Password     string         `yaml:"password"`
	DB           int            `yaml:"db"`
	PoolSize     int            `yaml:"poolSize"`
	MinIdleConns int            `yaml:"minIdleConns"`
	DialTimeout  time.Duration  `yaml:"dialTimeout"`
	ReadTimeout  time.Duration  `yaml:"readTimeout"`
	WriteTimeout time.Duration  `yaml:"writeTimeout"`
	PoolTimeout  time.Duration  `yaml:"poolTimeout"`
	IdleTimeout  time.Duration  `yaml:"idleTimeout"`
	KeyPrefix    string         `yaml:"keyPrefix"`
	MaxRetries   int            `yaml:"maxRetries"`
	TLS          RedisTLSConfig `yaml:"tls"`
}

// RedisTLSConfig contains Redis TLS settings
type RedisTLSConfig struct {
	Enabled            bool `yaml:"enabled"`
	InsecureSkipVerify bool `yaml:"insecureSkipVerify"`
}

// MemcachedConfig contains Memcached settings
type MemcachedConfig struct {
	Servers      []string      `yaml:"servers"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxIdleConns int           `yaml:"maxIdleConns"`
}

// QueueConfig contains message queue settings
type QueueConfig struct {
	Enabled      bool               `yaml:"enabled"`
	Provider     string             `yaml:"provider"`
	BufferSize   int                `yaml:"bufferSize"`
	RedisStreams RedisStreamsConfig `yaml:"redis-streams"`
	Kafka        KafkaConfig        `yaml:"kafka"`
	NATS         NATSConfig         `yaml:"nats"`
}

// RedisStreamsConfig contains Redis Streams settings
type RedisStreamsConfig struct {
	Address       string        `yaml:"address"`
	Password      string        `yaml:"password"`
	DB            int           `yaml:"db"`
	Stream        string        `yaml:"stream"`
	ConsumerGroup string        `yaml:"consumerGroup"`
	Consumer      string        `yaml:"consumer"`
	BlockTimeout  time.Duration `yaml:"blockTimeout"`
	BatchSize     int           `yaml:"batchSize"`
}

// KafkaConfig contains Kafka settings
type KafkaConfig struct {
	Brokers         []string        `yaml:"brokers"`
	Topic           string          `yaml:"topic"`
	GroupID         string          `yaml:"groupId"`
	AutoOffsetReset string          `yaml:"autoOffsetReset"`
	SessionTimeout  time.Duration   `yaml:"sessionTimeout"`
	TLS             KafkaTLSConfig  `yaml:"tls"`
	SASL            KafkaSASLConfig `yaml:"sasl"`
}

// KafkaTLSConfig contains Kafka TLS settings
type KafkaTLSConfig struct {
	Enabled bool `yaml:"enabled"`
}

// KafkaSASLConfig contains Kafka SASL settings
type KafkaSASLConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Mechanism string `yaml:"mechanism"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
}

// NATSConfig contains NATS settings
type NATSConfig struct {
	URL         string        `yaml:"url"`
	Subject     string        `yaml:"subject"`
	Queue       string        `yaml:"queue"`
	DurableName string        `yaml:"durableName"`
	TLS         NATSTLSConfig `yaml:"tls"`
}

// NATSTLSConfig contains NATS TLS settings
type NATSTLSConfig struct {
	Enabled bool `yaml:"enabled"`
}

// RegistryConfig contains registry adapter settings
type RegistryConfig struct {
	Name         string                 `yaml:"name"`
	Type         string                 `yaml:"type"`
	Enabled      bool                   `yaml:"enabled"`
	Priority     int                    `yaml:"priority"`
	Timeout      time.Duration          `yaml:"timeout"`
	Retries      int                    `yaml:"retries"`
	RetryBackoff time.Duration          `yaml:"retryBackoff"`
	Config       map[string]interface{} `yaml:"config"`
}

// TrustConfig contains trust and verification settings
type TrustConfig struct {
	Provider     string             `yaml:"provider"`
	Verification VerificationConfig `yaml:"verification"`
	File         FileTrustConfig    `yaml:"file"`
	Vault        VaultConfig        `yaml:"vault"`
	K8sSecret    K8sSecretConfig    `yaml:"k8s-secret"`
}

// VerificationConfig contains verification settings
type VerificationConfig struct {
	Enabled                 bool          `yaml:"enabled"`
	RequireSignature        bool          `yaml:"requireSignature"`
	RequireMerkleProof      bool          `yaml:"requireMerkleProof"`
	CheckRevocation         bool          `yaml:"checkRevocation"`
	AllowExpiredGracePeriod time.Duration `yaml:"allowExpiredGracePeriod"`
	OCSP                    OCSPConfig    `yaml:"ocsp"`
	CRL                     CRLConfig     `yaml:"crl"`
}

// OCSPConfig contains OCSP settings
type OCSPConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Timeout      time.Duration `yaml:"timeout"`
	CacheTimeout time.Duration `yaml:"cacheTimeout"`
}

// CRLConfig contains CRL settings
type CRLConfig struct {
	Enabled      bool          `yaml:"enabled"`
	CacheTimeout time.Duration `yaml:"cacheTimeout"`
}

// FileTrustConfig contains file-based trust store settings
type FileTrustConfig struct {
	TrustedRootsFile      string        `yaml:"trustedRootsFile"`
	TrustedRegistrarsFile string        `yaml:"trustedRegistrarsFile"`
	RefreshInterval       time.Duration `yaml:"refreshInterval"`
}

// VaultConfig contains Vault trust store settings
type VaultConfig struct {
	Address       string `yaml:"address"`
	AuthMethod    string `yaml:"authMethod"`
	Token         string `yaml:"token"`
	Role          string `yaml:"role"`
	SecretPath    string `yaml:"secretPath"`
	TLSSkipVerify bool   `yaml:"tlsSkipVerify"`
}

// K8sSecretConfig contains Kubernetes secret trust store settings
type K8sSecretConfig struct {
	Namespace     string `yaml:"namespace"`
	SecretName    string `yaml:"secretName"`
	RootsKey      string `yaml:"rootsKey"`
	RegistrarsKey string `yaml:"registrarsKey"`
}

// PolicyConfig contains policy engine settings
type PolicyConfig struct {
	Enabled       bool         `yaml:"enabled"`
	DefaultAction string       `yaml:"defaultAction"`
	Rules         []PolicyRule `yaml:"rules"`
}

// PolicyRule represents a single policy rule
type PolicyRule struct {
	Name    string   `yaml:"name"`
	Enabled bool     `yaml:"enabled"`
	Type    string   `yaml:"type"`
	Field   string   `yaml:"field"`
	Values  []string `yaml:"values"`
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	Enabled      bool                      `yaml:"enabled"`
	Provider     string                    `yaml:"provider"`
	KeyExtractor string                    `yaml:"keyExtractor"`
	Limits       map[string]RateLimitEntry `yaml:"limits"`
	Redis        RateLimitRedisConfig      `yaml:"redis"`
}

// RateLimitEntry represents a rate limit configuration
type RateLimitEntry struct {
	Requests int           `yaml:"requests"`
	Window   time.Duration `yaml:"window"`
}

// RateLimitRedisConfig contains Redis rate limit settings
type RateLimitRedisConfig struct {
	Address   string `yaml:"address"`
	KeyPrefix string `yaml:"keyPrefix"`
}

// TelemetryConfig contains observability settings
type TelemetryConfig struct {
	Logging LoggingConfig `yaml:"logging"`
	Metrics MetricsConfig `yaml:"metrics"`
	Tracing TracingConfig `yaml:"tracing"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string        `yaml:"level"`
	Format string        `yaml:"format"`
	Output string        `yaml:"output"`
	File   LogFileConfig `yaml:"file"`
}

// LogFileConfig contains log file settings
type LogFileConfig struct {
	Path       string `yaml:"path"`
	MaxSize    int    `yaml:"maxSize"`
	MaxBackups int    `yaml:"maxBackups"`
	MaxAge     int    `yaml:"maxAge"`
	Compress   bool   `yaml:"compress"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Path      string `yaml:"path"`
	Namespace string `yaml:"namespace"`
	Subsystem string `yaml:"subsystem"`
}

// TracingConfig contains tracing settings
type TracingConfig struct {
	Enabled     bool         `yaml:"enabled"`
	Provider    string       `yaml:"provider"`
	SampleRate  float64      `yaml:"sampleRate"`
	ServiceName string       `yaml:"serviceName"`
	OTLP        OTLPConfig   `yaml:"otlp"`
	Jaeger      JaegerConfig `yaml:"jaeger"`
}

// OTLPConfig contains OTLP exporter settings
type OTLPConfig struct {
	Endpoint string `yaml:"endpoint"`
	Insecure bool   `yaml:"insecure"`
	Headers  string `yaml:"headers"`
}

// JaegerConfig contains Jaeger exporter settings
type JaegerConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AgentHost string `yaml:"agentHost"`
	AgentPort int    `yaml:"agentPort"`
}

// envVarPattern matches ${VAR_NAME:default_value} or ${VAR_NAME}
var envVarPattern = regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)

// Load reads and parses the configuration file with environment variable substitution
func Load(path string) (*Config, error) {
	// Read the config file
	// #nosec G304 -- Config file path is provided by user/admin, not untrusted input
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables
	expanded := substituteEnvVars(string(data))

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	setDefaults(&cfg)

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// substituteEnvVars replaces ${VAR:default} patterns with environment variable values
func substituteEnvVars(input string) string {
	return envVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name and default value
		submatches := envVarPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		varName := submatches[1]
		defaultValue := ""
		if len(submatches) > 2 {
			defaultValue = submatches[2]
		}

		// Get environment variable value
		if value, exists := os.LookupEnv(varName); exists {
			return value
		}

		return defaultValue
	})
}

// setDefaults sets default values for unspecified configuration options
//
//nolint:gocyclo // High complexity acceptable for comprehensive configuration defaults
func setDefaults(cfg *Config) {
	// Server defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 120 * time.Second
	}
	if cfg.Server.MaxHeaderBytes == 0 {
		cfg.Server.MaxHeaderBytes = 1 << 20 // 1MB
	}
	if cfg.Server.GracefulShutdownTimeout == 0 {
		cfg.Server.GracefulShutdownTimeout = 30 * time.Second
	}

	const defaultCacheProvider = "memory"

	// Cache defaults
	if cfg.Cache.Provider == "" {
		cfg.Cache.Provider = defaultCacheProvider
	}
	if cfg.Cache.TTL.Default == 0 {
		cfg.Cache.TTL.Default = 5 * time.Minute
	}
	if cfg.Cache.TTL.Verified == 0 {
		cfg.Cache.TTL.Verified = 10 * time.Minute
	}
	if cfg.Cache.TTL.Revoked == 0 {
		cfg.Cache.TTL.Revoked = 1 * time.Hour
	}
	if cfg.Cache.TTL.NotFound == 0 {
		cfg.Cache.TTL.NotFound = 1 * time.Minute
	}
	if cfg.Cache.Memory.MaxSize == 0 {
		cfg.Cache.Memory.MaxSize = 10000
	}
	if cfg.Cache.Memory.CleanupInterval == 0 {
		cfg.Cache.Memory.CleanupInterval = 1 * time.Minute
	}

	// Queue defaults
	if cfg.Queue.Provider == "" {
		cfg.Queue.Provider = "memory"
	}
	if cfg.Queue.BufferSize == 0 {
		cfg.Queue.BufferSize = 1000
	}
	// Queue is disabled by default as it's not implemented yet
	// Set queue.enabled: true when queue processing is ready

	// Trust defaults
	if cfg.Trust.Provider == "" {
		cfg.Trust.Provider = "file"
	}

	// Policy defaults
	if cfg.Policy.DefaultAction == "" {
		cfg.Policy.DefaultAction = "allow"
	}

	// Rate limit defaults
	if cfg.RateLimit.Provider == "" {
		cfg.RateLimit.Provider = "memory"
	}
	if cfg.RateLimit.KeyExtractor == "" {
		cfg.RateLimit.KeyExtractor = "ip"
	}

	// Telemetry defaults
	if cfg.Telemetry.Logging.Level == "" {
		cfg.Telemetry.Logging.Level = "info"
	}
	if cfg.Telemetry.Logging.Format == "" {
		cfg.Telemetry.Logging.Format = "json"
	}
	if cfg.Telemetry.Metrics.Port == 0 {
		cfg.Telemetry.Metrics.Port = 9091
	}
	if cfg.Telemetry.Metrics.Path == "" {
		cfg.Telemetry.Metrics.Path = "/metrics"
	}
	if cfg.Telemetry.Tracing.SampleRate == 0 {
		cfg.Telemetry.Tracing.SampleRate = 0.1
	}
	if cfg.Telemetry.Tracing.ServiceName == "" {
		cfg.Telemetry.Tracing.ServiceName = "ans-resolver"
	}
}

// validate checks that the configuration is valid
func validate(cfg *Config) error {
	var errors []string

	// Validate server settings
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errors = append(errors, "server port must be between 1 and 65535")
	}

	// Validate cache provider
	validCacheProviders := map[string]bool{"memory": true, "redis": true, "memcached": true}
	if !validCacheProviders[cfg.Cache.Provider] {
		errors = append(errors, fmt.Sprintf("invalid cache provider: %s", cfg.Cache.Provider))
	}

	// Validate queue provider (only if queue is enabled)
	if cfg.Queue.Enabled {
		validQueueProviders := map[string]bool{"memory": true, "redis-streams": true, "kafka": true, "nats": true}
		if !validQueueProviders[cfg.Queue.Provider] {
			errors = append(errors, fmt.Sprintf("invalid queue provider: %s", cfg.Queue.Provider))
		}
	}

	// Validate trust provider
	validTrustProviders := map[string]bool{"file": true, "vault": true, "k8s-secret": true, "mock": true}
	if !validTrustProviders[cfg.Trust.Provider] {
		errors = append(errors, fmt.Sprintf("invalid trust provider: %s", cfg.Trust.Provider))
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}
