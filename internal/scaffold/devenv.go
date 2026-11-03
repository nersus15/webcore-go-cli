package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GenerateDevEnv membuat ".env-<pkg>" (checklist semua env var config
// webcore, semua ter-comment dengan contoh default value-nya) dan
// "air-<pkg>.toml" (hot-reload, entrypoint ["./tmp/main", pkg], env_files
// menunjuk ke .env-<pkg> itu) — mengikuti pola persis air.toml/
// air-<service>.toml yang sudah ada di project nyata webcore.
//
// Daftar env var di .env-<pkg> diambil dari tabel statis webcoreEnvVars
// (disalin dari infra/config/binding.go + defaults.go webcore, BUKAN
// di-parse dari config.yaml project pemakai) — supaya hasilnya konsisten
// dan tidak tergantung isi/keberadaan file config tertentu di project.
func GenerateDevEnv(pkg, mainDir string, force bool) error {
	envPath := fmt.Sprintf(".env-%s", pkg)
	if err := WriteFile(envPath, []byte(renderEnvFile(pkg)), force); err != nil {
		return err
	}

	airContent, err := render(airTomlTmpl, map[string]string{
		"Pkg":     pkg,
		"MainDir": filepath.ToSlash(mainDir),
	})
	if err != nil {
		return fmt.Errorf("render air-%s.toml: %w", pkg, err)
	}
	airPath := fmt.Sprintf("air-%s.toml", pkg)
	return WriteFile(airPath, airContent, force)
}

type envVar struct {
	section string // header grup, dicetak sekali tiap kali berubah
	env     string
	def     string // contoh default value; "" = tipe map/kompleks, tidak ada representasi scalar
}

// webcoreEnvVars disalin dari infra/config/binding.go (nama env var) +
// infra/config/defaults.go (nilai default) webcore — versi yang dikasih
// langsung, termasuk field yang belum ter-publish ke proxy publik
// (mis. auth.session.public_client/cookie_name, app.rate_limit.window/
// backend/fail_open, kafka.auto_commit/auto_offset_store/
// assignment_strategy, server.proxy_header).
//
// Nilai list ([]string) direpresentasikan sebagai string dipisah koma
// (konvensi umum Viper utk override slice lewat env var) — cek kode kalau
// project punya decode hook kustom yang berbeda.
var webcoreEnvVars = []envVar{
	{"App", "APP_NAME", "webcore-go"},
	{"App", "APP_VERSION", "1.0.0"},
	{"App", "APP_ENVIRONMENT", "development"},
	{"App", "APP_FEATURES_LOGGING", "true"},
	{"App", "APP_FEATURES_RECOVERY", "false"},
	{"App", "APP_FEATURES_TRACING", "false"},
	{"App", "APP_FEATURES_METRICS", "false"},
	{"App", "APP_FEATURES_PROFILING", "false"},
	{"App", "APP_LOGGING_LEVEL", "info"},
	{"App", "APP_LOGGING_FORMAT", "json"},
	{"App", "APP_LOGGING_OUTPUT", "stdout"},
	{"App", "APP_LOGGING_REMOTE_DRIVER", "sentry"},
	{"App", "APP_LOGGING_REMOTE_URI", ""},
	{"App", "APP_LOGGING_REMOTE_SAMPLE_RATE", "1.0"},
	{"App", "APP_LOGGING_REMOTE_STRUCTURED", "true"},
	{"App", "APP_LOGGING_REMOTE_DEFAULT_TAGS", ""},
	{"App", "APP_LOGGING_REMOTE_DEFAULT_CONTEXTS", ""},
	{"App", "APP_SECURITY_HEADERS", "false"},
	{"App", "APP_ADDITIONAL_HEADERS", ""},
	{"App", "APP_CORS_ALLOW_ORIGINS", "*"},
	{"App", "APP_CORS_ALLOW_METHODS", "GET,POST,PUT,DELETE,OPTIONS"},
	{"App", "APP_CORS_ALLOW_HEADERS", "Origin,Content-Type,Accept,Authorization"},
	{"App", "APP_CORS_EXPOSE_HEADERS", "Content-Length"},
	{"App", "APP_CORS_ALLOW_CREDENTIALS", "true"},
	{"App", "APP_CORS_MAX_AGE", "24h"},
	{"App", "APP_RATE_LIMIT_ENABLED", "false"},
	{"App", "APP_RATE_LIMIT_MAX", "1000"},
	{"App", "APP_RATE_LIMIT_WINDOW", "1m"},
	{"App", "APP_RATE_LIMIT_BACKEND", "memory"},
	{"App", "APP_RATE_LIMIT_FAIL_OPEN", "true"},
	{"App", "APP_MODULE_BASE_PATH", "./libs"},
	{"App", "APP_MODULE_DISABLED", ""},

	{"Server", "SERVER_HOST", "0.0.0.0"},
	{"Server", "SERVER_PORT", "7272"},
	{"Server", "SERVER_PATH", "/api"},
	{"Server", "SERVER_READ_TIMEOUT", "30s"},
	{"Server", "SERVER_WRITE_TIMEOUT", "30s"},
	{"Server", "SERVER_PROXY_HEADER", ""},

	{"Auth", "AUTH_DIRECTORY", "."},
	{"Auth", "AUTH_CONTROL", "RBAC"},
	{"Auth", "AUTH_STORE", "yaml"},
	{"Auth", "AUTH_TYPE", "jwt"},
	{"Auth", "AUTH_SECRET_KEY", ""},
	{"Auth", "AUTH_SESSION_BACKEND", ""},
	{"Auth", "AUTH_SESSION_EXPIRES_IN", "2h"},
	{"Auth", "AUTH_SESSION_REFRESH_IN", "48h"},
	{"Auth", "AUTH_SESSION_CONTENT_TYPE", "application/x-www-form-urlencoded"},
	{"Auth", "AUTH_SESSION_PUBLIC_CLIENT", "false"},
	{"Auth", "AUTH_SESSION_COOKIE_NAME", ""},
	{"Auth", "AUTH_SESSION_USERNAME_KEY", ""},
	{"Auth", "AUTH_SESSION_PASSWORD_KEY", ""},
	{"Auth", "AUTH_API_KEY_HEADER", "X-API-Key"},
	{"Auth", "AUTH_API_KEY_NAME", ""},

	{"Database", "DATABASE_DRIVER", "postgres"},
	{"Database", "DATABASE_URI", ""},
	{"Database", "DATABASE_SCHEME", ""},
	{"Database", "DATABASE_HOST", ""},
	{"Database", "DATABASE_PORT", "5432"},
	{"Database", "DATABASE_USER", ""},
	{"Database", "DATABASE_PASSWORD", ""},
	{"Database", "DATABASE_NAME", ""},
	{"Database", "DATABASE_SCHEMA_NAME", "public"},
	{"Database", "DATABASE_SSL_MODE", "disable"},
	{"Database", "DATABASE_MAX_OPEN_CONNS", "100"},
	{"Database", "DATABASE_MAX_IDLE_CONNS", "10"},
	{"Database", "DATABASE_CONN_MAX_LIFETIME", "300s"},

	{"Memory", "MEMORY_ENABLED", "true"},
	{"Memory", "MEMORY_LIMIT", "0"},
	{"Memory", "MEMORY_EXPIRES_IN", "180s"},

	{"Redis", "REDIS_HOST", ""},
	{"Redis", "REDIS_PORT", "6379"},
	{"Redis", "REDIS_PASSWORD", ""},
	{"Redis", "REDIS_DB", "0"},

	{"Kafka", "KAFKA_ENABLED", "false"},
	{"Kafka", "KAFKA_BROKERS", ""},
	{"Kafka", "KAFKA_GROUP_ID", ""},
	{"Kafka", "KAFKA_TOPICS", ""},
	{"Kafka", "KAFKA_AUTO_OFFSET_RESET", "earliest"},
	{"Kafka", "KAFKA_AUTO_COMMIT", "true"},
	{"Kafka", "KAFKA_AUTO_OFFSET_STORE", "true"},
	{"Kafka", "KAFKA_ASSIGNMENT_STRATEGY", "range,roundrobin"},

	{"PubSub", "PUBSUB_PROJECT_ID", ""},
	{"PubSub", "PUBSUB_TOPIC", ""},
	{"PubSub", "PUBSUB_SUBSCRIPTION", ""},
	{"PubSub", "PUBSUB_NO_CREDENTIALS", "false"},
	{"PubSub", "PUBSUB_CREDENTIALS", ""},

	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_TYPE", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_PROJECT_ID", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_PRIVATE_KEY_ID", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_PRIVATE_KEY", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_CLIENT_EMAIL", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_CLIENT_ID", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_AUTH_URI", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_TOKEN_URI", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_AUTH_PROVIDER_X509_URL", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_CLIENT_X509_URL", ""},
	{"PubSub Google Credentials", "PUBSUB_CREDENTIALS_DATA_UNIVERSE_DOMAIN", ""},

	{"PubSub Consumer", "PUBSUB_CONSUMER_MAXMESSAGES", "10"},
	{"PubSub Consumer", "PUBSUB_CONSUMER_SLEEPTIME", "5s"},
	{"PubSub Consumer", "PUBSUB_CONSUMER_ACKTIMEOUT", "60s"},
	{"PubSub Consumer", "PUBSUB_CONSUMER_RETRYCOUNT", "3"},
	{"PubSub Consumer", "PUBSUB_CONSUMER_RETRYDELAY", "1s"},
	{"PubSub Consumer", "PUBSUB_CONSUMER_FLOWCONTROL_ENABLED", "true"},
	{"PubSub Consumer", "PUBSUB_CONSUMER_FLOWCONTROL_MAXMESSAGES", "1000"},
	{"PubSub Consumer", "PUBSUB_CONSUMER_FLOWCONTROL_MAXBYTES", "1000000"},

	{"PubSub Producer", "PUBSUB_PRODUCER_ENABLEORDERING", "false"},
	{"PubSub Producer", "PUBSUB_PRODUCER_BATCHSIZE", "100"},
	{"PubSub Producer", "PUBSUB_PRODUCER_ATTRIBUTES", ""},
}

func renderEnvFile(pkg string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Checklist override config webcore untuk service \"%s\", dev-only.\n", pkg)
	b.WriteString("# SEMUA baris sengaja di-comment — uncomment & sesuaikan yang relevan saja,\n")
	b.WriteString("# mis. AUTH_TYPE=none kalau service ini tidak butuh auth. Nilai di sebelah\n")
	b.WriteString("# kanan adalah DEFAULT bawaan webcore, bukan nilai aktif.\n")
	b.WriteString("#\n")
	b.WriteString("# Dipakai otomatis oleh air-" + pkg + ".toml lewat env_files.\n\n")

	section := ""
	for _, v := range webcoreEnvVars {
		if v.section != section {
			section = v.section
			fmt.Fprintf(&b, "# --- %s ---\n", section)
		}
		fmt.Fprintf(&b, "# %s=%s\n", v.env, v.def)
	}

	return b.String()
}

const airTomlTmpl = `env_files = [".env-{{.Pkg}}"]

[build]
cmd = "go build -o ./tmp/main ./{{.MainDir}}/main.go"
entrypoint = ["./tmp/main", "{{.Pkg}}"]
exclude_dir = ["go", "tmp", "tests", "vendor", "testdata"]
include_ext = ["go", "tpl", "tmpl", "html", "md", "css", "js"]
poll = true
poll_interval = 500

[log]
time = false
main_only = false
silent = false

[color]
main = "magenta"
watcher = "cyan"
build = "yellow"
runner = "green"
`
