package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	CORS      CORSConfig
	S3        S3Config
	Claude    ClaudeConfig
	Google    GoogleConfig
	Turnstile TurnstileConfig
	Midtrans  MidtransConfig
	Mail      MailConfig
	Telegram  TelegramConfig
}

type AppConfig struct {
	Port string
	Env  string
}

type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	PoolSize int
}

type JWTConfig struct {
	Secret string
	TTL    time.Duration
}

type CORSConfig struct {
	AllowOrigins string
}

type S3Config struct {
	AccessKey    string
	SecretKey    string
	Region       string
	Endpoint     string
	Bucket       string
	UsePathStyle bool
}

type ClaudeConfig struct {
	APIKey string
	Model  string
}

type GoogleConfig struct {
	ClientID string
}

type TurnstileConfig struct {
	SecretKey string
}

type MidtransConfig struct {
	ServerKey    string
	ClientKey    string
	IsProduction bool
}

type MailConfig struct {
	Mailer     string
	Host       string
	Port       string
	Username   string
	Password   string
	Encryption string
	FromEmail  string
	FromName   string
}

type TelegramConfig struct {
	BotToken      string
	WebhookSecret string
}

func Load() *Config {
	loadEnvFile()

	maxOpen := mustGetEnvInt("DB_MAX_OPEN_CONNS")
	maxIdle := mustGetEnvInt("DB_MAX_IDLE_CONNS")
	maxLife := mustGetEnvInt("DB_CONN_MAX_LIFETIME")
	redisDB := mustGetEnvInt("REDIS_DB")
	redisPool := mustGetEnvInt("REDIS_POOL_SIZE")
	jwtTTLHours := mustGetEnvInt("JWT_TTL_HOURS")

	return &Config{
		App: AppConfig{
			Port: mustGetEnv("APP_PORT"),
			Env:  mustGetEnv("APP_ENV"),
		},
		Database: DatabaseConfig{
			Host:            mustGetEnv("DB_HOST"),
			Port:            mustGetEnv("DB_PORT"),
			User:            mustGetEnv("DB_USER"),
			Password:        mustGetEnv("DB_PASSWORD"),
			Name:            mustGetEnv("DB_NAME"),
			SSLMode:         mustGetEnv("DB_SSLMODE"),
			MaxOpenConns:    maxOpen,
			MaxIdleConns:    maxIdle,
			ConnMaxLifetime: time.Duration(maxLife) * time.Minute,
		},
		Redis: RedisConfig{
			Host:     mustGetEnv("REDIS_HOST"),
			Port:     mustGetEnv("REDIS_PORT"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
			PoolSize: redisPool,
		},
		JWT: JWTConfig{
			Secret: mustGetEnv("JWT_SECRET"),
			TTL:    time.Duration(jwtTTLHours) * time.Hour,
		},
		CORS: CORSConfig{
			AllowOrigins: mustGetEnv("CORS_ORIGINS"),
		},
		S3: S3Config{
			AccessKey:    mustGetEnv("AWS_ACCESS_KEY_ID"),
			SecretKey:    mustGetEnv("AWS_SECRET_ACCESS_KEY"),
			Region:       mustGetEnv("AWS_DEFAULT_REGION"),
			Endpoint:     os.Getenv("AWS_ENDPOINT"),
			Bucket:       mustGetEnv("AWS_BUCKET"),
			UsePathStyle: os.Getenv("AWS_USE_PATH_STYLE_ENDPOINT") == "true",
		},
		Claude: ClaudeConfig{
			APIKey: mustGetEnv("CLAUDE_API_KEY"),
			Model:  getEnvOrDefault("CLAUDE_MODEL", "claude-sonnet-4-5"),
		},
		Google: GoogleConfig{
			ClientID: os.Getenv("GOOGLE_CLIENT_ID"),
		},
		Turnstile: TurnstileConfig{
			SecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
		},
		Midtrans: MidtransConfig{
			ServerKey:    os.Getenv("MIDTRANS_SERVER_KEY"),
			ClientKey:    os.Getenv("MIDTRANS_CLIENT_KEY"),
			IsProduction: os.Getenv("MIDTRANS_IS_PROD") == "true",
		},
		Mail: MailConfig{
			Mailer:     getEnvOrDefault("MAIL_MAILER", "smtp"),
			Host:       os.Getenv("MAIL_HOST"),
			Port:       getEnvOrDefault("MAIL_PORT", "587"),
			Username:   os.Getenv("MAIL_USERNAME"),
			Password:   os.Getenv("MAIL_PASSWORD"),
			Encryption: getEnvOrDefault("MAIL_ENCRYPTION", "tls"),
			FromEmail:  os.Getenv("MAIL_FROM_ADDRESS"),
			FromName:   getEnvOrDefault("MAIL_FROM_NAME", "SAKU"),
		},
		Telegram: TelegramConfig{
			BotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
			WebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		},
	}
}

func loadEnvFile() {
	if root := findProjectRoot(); root != "" {
		envPath := filepath.Join(root, "envs", ".env")
		if err := godotenv.Load(envPath); err == nil {
			log.Printf("config: loaded environment from %s", envPath)
			return
		}
	}
	for _, p := range []string{"envs/.env", "../envs/.env"} {
		if err := godotenv.Load(p); err == nil {
			return
		}
	}
	log.Println("config: .env file not found, using system environment variables")
}

func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func mustGetEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalf("config: required environment variable %s is not set", key)
	}
	return v
}

func mustGetEnvInt(key string) int {
	v := mustGetEnv(key)
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("config: invalid integer value for %s: %q", key, v)
	}
	return n
}
