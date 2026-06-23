package conf

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/alert666/api-server/base/constant"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	defaultLoglevel             = "info"
	defaultLogEncoder           = "console"
	defaultServerBind           = "0.0.0.0:8080"
	defaultServerTimeZone       = "Asia/Shanghai"
	defaultJwtIssuer            = "api-server"
	defaultJwtAccessExpireTime  = "15m"
	defaultJwtRefreshExpireTime = "168h"
	defaultRedisExpireTime      = "1h"
)

// 加载配置
func LoadConfig(configPath string) (err error) {
	_, err = os.Stat(configPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("configuration file %s does not exist", configPath)
	}
	if err != nil {
		return fmt.Errorf("stat configuration file %s faild. err: %w", configPath, err)
	}
	zap.L().Info("start loading configuration file", zap.String("path", configPath))
	configDir := filepath.Dir(configPath)
	configBase := filepath.Base(configPath)
	viper.SetConfigName(configBase)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)
	if err = viper.ReadInConfig(); err != nil {
		return fmt.Errorf("reading configuration files %s faild. err: %w", configPath, err)
	}
	return nil
}

// 获取全部配置
func AllConfig() map[string]any {
	return viper.AllSettings()
}

// 服务配置
func GetServerBind() string {
	bind := viper.GetString("server.bind")
	if bind == "" {
		bind = defaultServerBind
	}
	return bind
}

// GetGRPCBind 获取 gRPC 监听地址
func GetGRPCBind() string {
	bind := viper.GetString("grpc.bind")
	if bind == "" {
		bind = defaultGRPCBind
	}
	return bind
}

// GetGrpcTLSCertFile 获取 gRPC TLS 证书文件路径
func GetGrpcTLSCertFile() string {
	return viper.GetString("grpc.tls.certFile")
}

// GetGrpcTLSKeyFile 获取 gRPC TLS 私钥文件路径
func GetGrpcTLSKeyFile() string {
	return viper.GetString("grpc.tls.keyFile")
}

// GetGrpcTLSCAFile 获取 gRPC mTLS CA 证书文件路径（用于验证客户端证书）
// 配置此项后开启 mTLS（双向 TLS），客户端必须出示由该 CA 签发的证书
func GetGrpcTLSCAFile() string {
	return viper.GetString("grpc.tls.caFile")
}

func GetServerTimeZone() string {
	timeZone := viper.GetString("server.timeZone")
	if timeZone == "" {
		timeZone = defaultServerTimeZone
	}
	return timeZone
}

// 日志配置
func GetLogLevel() string {
	logLevel := viper.GetString("log.level")
	if logLevel == "" {
		logLevel = defaultLoglevel
	}
	return logLevel
}

func GetLogEncoder() string {
	logEncoder := viper.GetString("log.encoder")
	if logEncoder == "" {
		logEncoder = defaultLogEncoder
	}
	return logEncoder
}

// jwt 配置
func GetJwtSecret() (string, error) {
	secret := viper.GetString("jwt.secret")
	if secret == "" {
		return "", fmt.Errorf("jwt.secret is empty")
	}
	return secret, nil
}

func GetJwtIssuer() string {
	issuer := viper.GetString("jwt.issuer")
	if issuer == "" {
		zap.L().Info("jwt.issuer is empty, set default", zap.String("jwt.issuer", defaultJwtIssuer))
		return defaultJwtIssuer
	}
	return issuer
}

func GetJwtAccessExpirationTime() (time.Duration, error) {
	expireTime := viper.GetDuration("jwt.accessExpireTime")
	if expireTime == 0 {
		expire, err := time.ParseDuration(defaultJwtAccessExpireTime)
		if err != nil {
			return 0, fmt.Errorf("failed to parse jwt.accessExpireTime err: %v", err)
		}
		return expire, nil
	}
	return expireTime, nil
}

func GetJwtRefreshExpirationTime() (time.Duration, error) {
	expireTime := viper.GetDuration("jwt.refreshExpireTime")
	if expireTime == 0 {
		expire, err := time.ParseDuration(defaultJwtRefreshExpireTime)
		if err != nil {
			return 0, fmt.Errorf("failed to parse jwt.refreshExpireTime err: %v", err)
		}
		return expire, nil
	}
	return expireTime, nil
}

// mysql 配置
func GetMysqlDsn() (dsn string, err error) {
	user := viper.GetString("mysql.username")
	if user == "" {
		return "", fmt.Errorf("mysql.username is empty")
	}
	pas := viper.GetString("mysql.password")
	if pas == "" {
		return "", fmt.Errorf("mysql.password is empty")
	}
	host := viper.GetString("mysql.host")
	if host == "" {
		return "", fmt.Errorf("mysql.host is empty")
	}
	database := viper.GetString("mysql.database")
	if database == "" {
		return "", fmt.Errorf("mysql.database is empty")
	}
	dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local&timeout=10000ms",
		user,
		pas,
		host,
		database,
	)
	return dsn, nil
}

func GetMysqlMaxIdleConns() int {
	maxIdleConns := viper.GetInt("mysql.maxIdleConns")
	if maxIdleConns == 0 {
		return 10
	}
	return maxIdleConns
}

func GetMysqlMaxOpenConns() int {
	maxOpenConns := viper.GetInt("mysql.maxOpenConns")
	if maxOpenConns == 0 {
		return 30
	}
	return maxOpenConns
}

func GetMysqlMaxLifetime() time.Duration {
	maxLifetime := viper.GetDuration("mysql.maxLifetime")
	if maxLifetime == 0 {
		return 30 * time.Minute
	}
	return maxLifetime
}

// redis 配置
func GetRedisPoolSize() int {
	poolSize := viper.GetInt("redis.poolSize")
	if poolSize == 0 {
		return 50
	}
	return poolSize
}

func GetRedisMinIdleConns() int {
	minIdleConns := viper.GetInt("redis.minIdleConns")
	if minIdleConns == 0 {
		return 20
	}
	return minIdleConns
}

func GetRedisConnMaxLifetime() time.Duration {
	connMaxLifetime := viper.GetDuration("redis.connMaxLifetime")
	if connMaxLifetime == 0 {
		return 30 * time.Minute
	}
	return connMaxLifetime
}

func GetRedisUser() string {
	return viper.GetString("redis.username")
}

func GetRedisPassword() (string, error) {
	password := viper.GetString("redis.password")
	if password == "" {
		return "", fmt.Errorf("redis.password is empty")
	}
	return password, nil
}

func GetRedisMasterName() (string, error) {
	masterName := viper.GetString("redis.sentinel.masterName")
	if masterName == "" {
		return "", fmt.Errorf("redis.sentinel.masterName is empty")
	}
	return masterName, nil
}

func GetRedisSentinelPassword() (string, error) {
	sentPassword := viper.GetString("redis.sentinel.password")
	if sentPassword == "" {
		return "", fmt.Errorf("redis.sentinel.password is empty")
	}
	return sentPassword, nil
}

func GetRedisSentinelHosts() ([]string, error) {
	sentinelHosts := viper.GetStringSlice("redis.sentinel.hosts")
	if len(sentinelHosts) == 0 {
		return nil, fmt.Errorf("redis.sentinel.hosts is empty")
	}
	return sentinelHosts, nil
}

func GetRedisHost() (string, error) {
	host := viper.GetString("redis.host")
	if host == "" {
		return "", fmt.Errorf("redis.host is empty")
	}
	return host, nil
}

func GetRedisDB() int {
	return viper.GetInt("redis.db")
}

func GetRedisMode() string {
	return viper.GetString("redis.mode")
}

func GetRedisExpireTime() (time.Duration, error) {
	expireTime := viper.GetDuration("redis.expireTime")
	if expireTime == 0 {
		duration, err := time.ParseDuration(defaultRedisExpireTime)
		if err != nil {
			return 0, fmt.Errorf("failed to parser defaultRedisExpireTime err: %v", err)
		}
		zap.L().Info("redis.expireTime is empty, set default", zap.String("expireTime", defaultRedisExpireTime))
		return duration, nil
	}

	return expireTime, nil
}

func GetRedisKeyPrefix() (string, error) {
	prefix := viper.GetString("redis.keyPrefix")
	if prefix == "" {
		return "", fmt.Errorf("redis.keyPrefix is empty")
	}
	return prefix, nil
}

// GetAlertTenantKey 获取租户标签的键
func GetAlertTenantKey() string {
	if tenantKey := viper.GetString("alert.tenantKey"); tenantKey != "" {
		return tenantKey
	}
	return "cluster"
}

type ExtraSyncConf map[string]map[string][]string

func (e ExtraSyncConf) GetConfig(name string) (map[string][]string, error) {
	if config, ok := e[name]; ok {
		return config, nil
	}
	return nil, fmt.Errorf("name: %s, %w", name, constant.ErrExtraSyncConfNotFound)
}

// GetAlertExtraSync 获取额外同步配置
func GetAlertExtraSync() (*ExtraSyncConf, error) {
	extraSyncConf := viper.GetStringMap("alert.extraSync")
	if extraSyncConf == nil {
		return nil, nil
	}
	extraSyncConfBy, err := json.Marshal(&extraSyncConf)
	if err != nil {
		return nil, fmt.Errorf("alert.extraSync marshal failed, %w", err)
	}

	var e ExtraSyncConf
	if err := json.Unmarshal(extraSyncConfBy, &e); err != nil {
		return nil, fmt.Errorf("alert.extraSync umarshal failed, %w", err)
	}

	return &e, nil
}

// GetAlertReceiveToken 获取告警接收认证 token
// 配置后，Alertmanager webhook 请求必须携带 Authorization: Bearer <token>
// 未配置时打印警告日志，不校验认证
func GetAlertReceiveToken() string {
	token := viper.GetString("alert.receiveToken")
	if token == "" {
		zap.L().Warn("alert.receiveToken is empty, alert receiving endpoint is unprotected")
	}
	return token
}

func GetAlertRepeatInterval() time.Duration {
	return viper.GetDuration("alert.repeatInterval")
}

const defaultGRPCBind = "0.0.0.0:9090"

// GetInternalAdvertiseAddr 获取本实例的内部广播地址。
// 优先配置 internal.advertiseAddr；未配置时用出站 IP + server.bind 端口。
func GetInternalAdvertiseAddr() string {
	if addr := viper.GetString("internal.advertiseAddr"); addr != "" {
		return addr
	}

	_, port, err := net.SplitHostPort(GetServerBind())
	if err != nil {
		port = "8080"
	}

	ip := os.Getenv("MY_POD_IP")
	if ip != "" {
		return fmt.Sprintf("http://%s:%s", ip, port)
	}

	return fmt.Sprintf("http://%s:%s", GetOutboundIP(), port)
}

// GetOutboundIP returns the preferred outbound IP of this machine.
func GetOutboundIP() string {
	addrs := []string{"223.5.5.5:80", "223.6.6.6:80"}
	for _, addr := range addrs {
		conn, err := net.Dial("udp", addr)
		if err == nil {
			ip := conn.LocalAddr().(*net.UDPAddr).IP.String()
			conn.Close()
			return ip
		}
	}
	return "127.0.0.1"
}

// GetInternalToken 获取内部 API 认证 token
func GetInternalToken() string {
	return viper.GetString("internal.token")
}
