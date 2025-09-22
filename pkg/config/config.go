package config

type InfraConfig struct {
	DefaultTimezone string `cfg:"DEFAULT_TIMEZONE" default:"Asia/Shanghai"`
	AppSecret       string `cfg:"APP_SECRET" default:""`

	// 数据库配置
	MYSQL_HOST     string `cfg:"MYSQL_HOST"`
	MYSQL_PORT     int    `cfg:"MYSQL_PORT"`
	MYSQL_USER     string `cfg:"MYSQL_USER"`
	MYSQL_PASSWORD string `cfg:"MYSQL_PASSWORD"`
	MYSQL_DBNAME   string `cfg:"MYSQL_DBNAME"`

	// Redis配置
	RedisAddr     string `cfg:"REDIS_ADDR" default:"localhost:6379"`
	RedisPassword string `cfg:"REDIS_PASSWORD" default:""`
	RedisDB       int    `cfg:"REDIS_DB" default:"0"`

	// 邮件配置
	SendMail struct {
		Host     string `cfg:"HOST"`
		Port     int    `cfg:"PORT" default:"587"`
		Username string `cfg:"USERNAME"`
		Password string `cfg:"PASSWORD"`
		From     string `cfg:"MAILFROM" default:""`
		TLS      string `cfg:"TLS" default:"NONE"`
	} `cfg:"SMTP"`

	// 任务队列配置
	AsynqName struct {
		High    string `cfg:"HIGH" default:"project-high"`
		Low     string `cfg:"LOW" default:"project-low"`
		Default string `cfg:"DEFAULT" default:"project"`
	} `cfg:"ASYNQ_NAME"`

	// 存储配置
	PublicStorage  StorageInstanceConfig `cfg:"STORAGE_PUBLIC"`
	PrivateStorage StorageInstanceConfig `cfg:"STORAGE_PRIVATE"`
}

type StorageInstanceConfig struct {
	Type  string `cfg:"TYPE" default:"local"`
	Local struct {
		BasePath string `cfg:"BASE_PATH" default:"storage"`
		BaseURL  string `cfg:"BASE_URL" default:"/storage"`
	} `cfg:"LOCAL"`
	S3 struct {
		AccessKey string `cfg:"ACCESS_KEY"`
		SecretKey string `cfg:"SECRET_KEY"`
		Bucket    string `cfg:"BUCKET"`
		Region    string `cfg:"REGION"`
		Endpoint  string `cfg:"ENDPOINT"`
		PublicURL string `cfg:"PUBLIC_URL"`
	} `cfg:"S3"`
}

var Config *InfraConfig
