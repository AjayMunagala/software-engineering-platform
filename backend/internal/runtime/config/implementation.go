package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type loader struct{}

// NewLoader creates the side-effect-free configuration loader. Files are read
// only when an explicit configuration path is supplied.
func NewLoader() Loader { return loader{} }

func (loader) Load(ctx context.Context, request LoadRequest) (LoadedConfiguration, error) {
	if ctx == nil {
		return LoadedConfiguration{}, newError(ErrorInvalidInput, "context", nil)
	}
	if err := ctx.Err(); err != nil {
		return LoadedConfiguration{}, newError(ErrorCanceled, "context", err)
	}

	environment, environmentSecrets, environmentConfigFile, err := parseEnvironment(request.environment)
	if err != nil {
		return LoadedConfiguration{}, err
	}
	arguments, argumentConfigFile, err := parseArguments(request.arguments)
	if err != nil {
		return LoadedConfiguration{}, err
	}
	configFile := environmentConfigFile
	if argumentConfigFile != "" {
		configFile = argumentConfigFile
	}

	fileValues := map[string]string{}
	if configFile != "" {
		fileValues, err = readConfigurationFile(ctx, configFile)
		if err != nil {
			return LoadedConfiguration{}, err
		}
	}

	profile, err := selectProfile(fileValues, environment, arguments)
	if err != nil {
		return LoadedConfiguration{}, err
	}
	values, sources := defaultValues(profile)
	applyValues(values, sources, fileValues, SourceFile)
	applyValues(values, sources, environment, SourceEnvironment)
	applyValues(values, sources, arguments, SourceCommandLine)

	configuration, err := buildRuntimeConfig(values, sources)
	if err != nil {
		return LoadedConfiguration{}, err
	}

	provider := request.secretProvider
	if provider != nil && len(environmentSecrets) != 0 {
		return LoadedConfiguration{}, newError(ErrorSecretAmbiguous, "secret_provider", nil)
	}
	if provider == nil {
		provider = &environmentSecretProvider{values: environmentSecrets}
	}
	loaded := LoadedConfiguration{configuration: configuration, provider: provider}
	if err := loaded.ValidateSecrets(ctx); err != nil {
		return LoadedConfiguration{}, err
	}
	return loaded, nil
}

func applyValues(target map[string]string, sources map[string]SourceCategory, values map[string]string, source SourceCategory) {
	for field, value := range values {
		target[field] = value
		sources[field] = source
	}
}

func selectProfile(fileValues, environment, arguments map[string]string) (Profile, error) {
	value := string(ProfileLocal)
	if candidate, ok := fileValues[fieldProfile]; ok {
		value = candidate
	}
	if candidate, ok := environment[fieldProfile]; ok {
		value = candidate
	}
	if candidate, ok := arguments[fieldProfile]; ok {
		value = candidate
	}
	profile := Profile(value)
	if !profile.valid() {
		return "", newError(ErrorInvalidProfile, fieldProfile, nil)
	}
	return profile, nil
}

func parseEnvironment(entries []string) (map[string]string, map[SecretReference][]byte, string, error) {
	values := make(map[string]string)
	secrets := make(map[SecretReference][]byte)
	seen := make(map[string]struct{})
	configFile := ""
	for _, entry := range entries {
		name, value, found := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, "AEGIS_") {
			continue
		}
		if !found || name == "" {
			return nil, nil, "", newError(ErrorInvalidInput, "environment", nil)
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, nil, "", newError(ErrorDuplicateField, environmentFieldName(name), nil)
		}
		seen[name] = struct{}{}
		if name == "AEGIS_CONFIG_FILE" {
			configFile = value
			continue
		}
		if reference, secret := secretEnvironmentFields[name]; secret {
			secrets[reference] = []byte(value)
			continue
		}
		field, known := environmentFields[name]
		if !known {
			return nil, nil, "", newError(ErrorUnknownField, "environment", nil)
		}
		values[field] = value
	}
	return values, secrets, configFile, nil
}

func environmentFieldName(name string) string {
	value := strings.ToLower(strings.TrimPrefix(name, "AEGIS_"))
	if safeField(value) {
		return value
	}
	return "environment"
}

func parseArguments(arguments []string) (map[string]string, string, error) {
	values := make(map[string]string)
	seen := make(map[string]struct{})
	configFile := ""
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if !strings.HasPrefix(argument, "--") || argument == "--" {
			return nil, "", newError(ErrorInvalidInput, "arguments", nil)
		}
		nameValue := strings.TrimPrefix(argument, "--")
		name, value, hasValue := strings.Cut(nameValue, "=")
		if !hasValue {
			index++
			if index >= len(arguments) || strings.HasPrefix(arguments[index], "--") {
				return nil, "", newError(ErrorInvalidInput, "arguments", nil)
			}
			value = arguments[index]
		}
		if name == "" || value == "" {
			return nil, "", newError(ErrorInvalidInput, "arguments", nil)
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, "", newError(ErrorDuplicateField, commandFieldName(name), nil)
		}
		seen[name] = struct{}{}
		if name == "config-file" {
			configFile = value
			continue
		}
		field, known := commandFields[name]
		if !known {
			return nil, "", newError(ErrorUnknownField, "arguments", nil)
		}
		values[field] = value
	}
	return values, configFile, nil
}

func commandFieldName(name string) string {
	value := strings.ReplaceAll(name, "-", "_")
	if safeField(value) {
		return value
	}
	return "arguments"
}

func readConfigurationFile(ctx context.Context, path string) (map[string]string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, newError(ErrorInvalidValue, "config_file", nil)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, newError(ErrorFileRead, "config_file", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maximumConfigBytes {
		return nil, newError(ErrorInvalidValue, "config_file", nil)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, newError(ErrorFileRead, "config_file", err)
	}
	defer file.Close()
	if err := ctx.Err(); err != nil {
		return nil, newError(ErrorCanceled, "context", err)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumConfigBytes+1))
	if err != nil {
		return nil, newError(ErrorFileRead, "config_file", err)
	}
	if len(data) > maximumConfigBytes || !utf8.Valid(data) {
		return nil, newError(ErrorInvalidValue, "config_file", nil)
	}
	return decodeStrictConfiguration(data)
}

func decodeStrictConfiguration(data []byte) (map[string]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return nil, newError(ErrorInvalidValue, "config_file", err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return nil, newError(ErrorInvalidValue, "config_file", nil)
	}
	values := make(map[string]string)
	if err := decodeObject(decoder, "", values); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, newError(ErrorInvalidValue, "config_file", err)
	}
	return values, nil
}

func decodeObject(decoder *json.Decoder, prefix string, values map[string]string) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return newError(ErrorInvalidValue, "config_file", err)
		}
		name, ok := token.(string)
		if !ok || name == "" {
			return newError(ErrorInvalidValue, "config_file", nil)
		}
		if _, duplicate := seen[name]; duplicate {
			return newError(ErrorDuplicateField, joinField(prefix, name), nil)
		}
		seen[name] = struct{}{}
		field := joinField(prefix, name)
		if expected, leaf := fileFields[field]; leaf {
			valueToken, err := decoder.Token()
			if err != nil {
				return newError(ErrorInvalidValue, field, err)
			}
			value, valid := tokenString(valueToken, expected)
			if !valid {
				return newError(ErrorInvalidValue, field, nil)
			}
			values[field] = value
			continue
		}
		if !objectPrefix(field) {
			return newError(ErrorUnknownField, field, nil)
		}
		open, err := decoder.Token()
		if err != nil {
			return newError(ErrorInvalidValue, field, err)
		}
		delimiter, ok := open.(json.Delim)
		if !ok || delimiter != '{' {
			return newError(ErrorInvalidValue, field, nil)
		}
		if err := decodeObject(decoder, field, values); err != nil {
			return err
		}
	}
	closeToken, err := decoder.Token()
	if err != nil {
		return newError(ErrorInvalidValue, "config_file", err)
	}
	closeDelimiter, ok := closeToken.(json.Delim)
	if !ok || closeDelimiter != '}' {
		return newError(ErrorInvalidValue, "config_file", nil)
	}
	return nil
}

func joinField(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func objectPrefix(field string) bool {
	prefix := field + "."
	for candidate := range fileFields {
		if strings.HasPrefix(candidate, prefix) {
			return true
		}
	}
	return false
}

func tokenString(token any, expected valueType) (string, bool) {
	switch expected {
	case valueString:
		value, ok := token.(string)
		return value, ok
	case valueInteger:
		value, ok := token.(json.Number)
		return string(value), ok
	case valueBoolean:
		value, ok := token.(bool)
		return strconv.FormatBool(value), ok
	default:
		return "", false
	}
}

func buildRuntimeConfig(values map[string]string, sources map[string]SourceCategory) (RuntimeConfig, error) {
	profile := Profile(values[fieldProfile])
	if !profile.valid() {
		return RuntimeConfig{}, newError(ErrorInvalidProfile, fieldProfile, nil)
	}

	startupTimeout, err := parseDuration(values, fieldStartupTimeout, time.Second, 10*time.Minute)
	if err != nil {
		return RuntimeConfig{}, err
	}
	drainTimeout, err := parseDuration(values, fieldDrainTimeout, time.Second, 10*time.Minute)
	if err != nil {
		return RuntimeConfig{}, err
	}
	forcedTimeout, err := parseDuration(values, fieldForcedTimeout, time.Second, time.Minute)
	if err != nil {
		return RuntimeConfig{}, err
	}
	connectTimeout, err := parseDuration(values, fieldDBConnect, time.Second, time.Minute)
	if err != nil {
		return RuntimeConfig{}, err
	}
	healthTimeout, err := parseDuration(values, fieldHealthTimeout, 100*time.Millisecond, time.Minute)
	if err != nil {
		return RuntimeConfig{}, err
	}
	healthInterval, err := parseDuration(values, fieldHealthInterval, time.Second, 10*time.Minute)
	if err != nil {
		return RuntimeConfig{}, err
	}
	metricsInterval, err := parseDuration(values, fieldMetricsInterval, time.Second, time.Hour)
	if err != nil {
		return RuntimeConfig{}, err
	}
	maxLifetime, err := parseDuration(values, fieldMaxLifetime, time.Minute, 30*24*time.Hour)
	if err != nil {
		return RuntimeConfig{}, err
	}
	lifetimeJitter, err := parseDuration(values, fieldLifetimeJitter, time.Second, 24*time.Hour)
	if err != nil {
		return RuntimeConfig{}, err
	}
	maxIdle, err := parseDuration(values, fieldMaxIdle, time.Minute, 30*24*time.Hour)
	if err != nil {
		return RuntimeConfig{}, err
	}
	poolHealth, err := parseDuration(values, fieldPoolHealth, time.Second, 10*time.Minute)
	if err != nil {
		return RuntimeConfig{}, err
	}
	pingTimeout, err := parseDuration(values, fieldPingTimeout, 100*time.Millisecond, time.Minute)
	if err != nil {
		return RuntimeConfig{}, err
	}
	if lifetimeJitter > maxLifetime {
		return RuntimeConfig{}, newError(ErrorConflictingValue, fieldLifetimeJitter, nil)
	}

	port, err := parseInteger(values, fieldDBPort, 1, 65535)
	if err != nil {
		return RuntimeConfig{}, err
	}
	budget, err := parseInteger(values, fieldDBBudget, 1, 64)
	if err != nil {
		return RuntimeConfig{}, err
	}
	failureThreshold, err := parseInteger(values, fieldHealthThreshold, 1, 100)
	if err != nil {
		return RuntimeConfig{}, err
	}
	metricsEnabled, err := strconv.ParseBool(values[fieldMetricsEnabled])
	if err != nil {
		return RuntimeConfig{}, newError(ErrorInvalidValue, fieldMetricsEnabled, err)
	}

	if err := validateHost(values[fieldDBHost]); err != nil {
		return RuntimeConfig{}, err
	}
	if !safeIdentifier(values[fieldDBName]) {
		return RuntimeConfig{}, newError(ErrorInvalidValue, fieldDBName, nil)
	}
	if !safeApplicationName(values[fieldDBApplication]) {
		return RuntimeConfig{}, newError(ErrorInvalidValue, fieldDBApplication, nil)
	}
	level := LogLevel(values[fieldLogLevel])
	if !level.valid() {
		return RuntimeConfig{}, newError(ErrorInvalidValue, fieldLogLevel, nil)
	}
	if (profile == ProfileStaging || profile == ProfileProduction) && level == LogDebug {
		return RuntimeConfig{}, newError(ErrorUnsupported, fieldLogLevel, nil)
	}
	format := LogFormat(values[fieldLogFormat])
	if !format.valid() {
		return RuntimeConfig{}, newError(ErrorInvalidValue, fieldLogFormat, nil)
	}
	if profile != ProfileLocal && format != LogJSON {
		return RuntimeConfig{}, newError(ErrorUnsupported, fieldLogFormat, nil)
	}
	if !safeServiceName(values[fieldService]) {
		return RuntimeConfig{}, newError(ErrorInvalidValue, fieldService, nil)
	}

	poolMode := PoolSeparate
	if profile == ProfileLocal || profile == ProfileCI {
		poolMode = PoolCombined
	}
	pools, err := buildPools(values, sources, profile, poolMode, int32(budget))
	if err != nil {
		return RuntimeConfig{}, err
	}
	tls, err := buildTLS(values, profile)
	if err != nil {
		return RuntimeConfig{}, err
	}

	return RuntimeConfig{
		profile: profile,
		startup: StartupConfig{startupTimeout: startupTimeout, drainTimeout: drainTimeout, forcedShutdownTimeout: forcedTimeout},
		logging: LoggingConfig{level: level, format: format, serviceName: values[fieldService]},
		database: DatabaseConfig{
			host: values[fieldDBHost], port: uint16(port), name: values[fieldDBName],
			user: values[fieldDBUser], applicationName: values[fieldDBApplication],
			connectTimeout: connectTimeout, connectionBudget: int32(budget),
			pools: PoolSetConfig{
				mode: poolMode, ingest: pools[0], read: pools[1], retention: pools[2],
				maxConnLifetime: maxLifetime, maxConnLifetimeJitter: lifetimeJitter,
				maxConnIdleTime: maxIdle, healthCheckPeriod: poolHealth, pingTimeout: pingTimeout,
			},
			tls: tls,
		},
		health:        HealthConfig{checkTimeout: healthTimeout, checkInterval: healthInterval, failureThreshold: int(failureThreshold)},
		observability: ObservabilityConfig{metricsEnabled: metricsEnabled, collectionInterval: metricsInterval},
		sources:       cloneSources(sources),
	}, nil
}

func buildPools(values map[string]string, sources map[string]SourceCategory, profile Profile, mode PoolMode, budget int32) ([3]PoolConfig, error) {
	var result [3]PoolConfig
	fields := [3][3]string{
		{fieldIngestUser, fieldIngestMax, fieldIngestMin},
		{fieldReadUser, fieldReadMax, fieldReadMin},
		{fieldRetainUser, fieldRetainMax, fieldRetainMin},
	}
	references := [3]SecretReference{SecretDatabaseIngest, SecretDatabaseRead, SecretDatabaseRetention}
	if mode == PoolCombined {
		if budget != 4 {
			return result, newError(ErrorUnsupported, fieldDBBudget, nil)
		}
		user := values[fieldDBUser]
		if !safeIdentifier(user) {
			return result, newError(ErrorInvalidValue, fieldDBUser, nil)
		}
		var maximum, minimum int64
		for index, names := range fields {
			parsedMax, err := parseInteger(values, names[1], 1, int64(budget))
			if err != nil {
				return result, err
			}
			parsedMin, err := parseInteger(values, names[2], 0, parsedMax)
			if err != nil {
				return result, err
			}
			if index == 0 {
				maximum, minimum = parsedMax, parsedMin
			}
			if parsedMax != maximum || parsedMin != minimum {
				return result, newError(ErrorConflictingValue, "database.pools", nil)
			}
			if values[names[0]] != "" {
				return result, newError(ErrorUnsupported, names[0], nil)
			}
			result[index] = PoolConfig{user: user, maxConns: int32(maximum), minIdleConns: int32(minimum), secret: SecretDatabaseCombined}
		}
		return result, nil
	}

	if values[fieldDBUser] != "" {
		return result, newError(ErrorUnsupported, fieldDBUser, nil)
	}
	if sources[fieldDBBudget] == SourceDefault {
		return result, newError(ErrorInvalidValue, fieldDBBudget, nil)
	}
	var sum int64
	users := make(map[string]struct{}, 3)
	for index, names := range fields {
		for _, field := range names {
			if sources[field] == SourceDefault {
				return result, newError(ErrorInvalidValue, field, nil)
			}
		}
		user := values[names[0]]
		if !safeIdentifier(user) {
			return result, newError(ErrorInvalidValue, names[0], nil)
		}
		if _, duplicate := users[user]; duplicate {
			return result, newError(ErrorConflictingValue, names[0], nil)
		}
		users[user] = struct{}{}
		maximum, err := parseInteger(values, names[1], 1, int64(budget))
		if err != nil {
			return result, err
		}
		minimum, err := parseInteger(values, names[2], 0, maximum)
		if err != nil {
			return result, err
		}
		sum += maximum
		result[index] = PoolConfig{user: user, maxConns: int32(maximum), minIdleConns: int32(minimum), secret: references[index]}
	}
	if sum > int64(budget) {
		return result, newError(ErrorConflictingValue, fieldDBBudget, nil)
	}
	return result, nil
}

func buildTLS(values map[string]string, profile Profile) (TLSConfig, error) {
	mode := TLSMode(values[fieldTLSMode])
	if !mode.valid() {
		return TLSConfig{}, newError(ErrorInvalidValue, fieldTLSMode, nil)
	}
	host := values[fieldDBHost]
	if mode == TLSDisabled {
		if profile != ProfileLocal && profile != ProfileCI {
			return TLSConfig{}, newError(ErrorUnsupported, fieldTLSMode, nil)
		}
		if !loopbackOrSocket(host) {
			return TLSConfig{}, newError(ErrorConflictingValue, fieldDBHost, nil)
		}
	} else if (profile == ProfileStaging || profile == ProfileProduction) && mode != TLSVerifyFull {
		return TLSConfig{}, newError(ErrorUnsupported, fieldTLSMode, nil)
	}
	rootCA := values[fieldTLSRootCA]
	clientCert := values[fieldTLSClientCert]
	clientKey := values[fieldTLSClientKey]
	serverName := values[fieldTLSServerName]
	if (clientCert == "") != (clientKey == "") {
		return TLSConfig{}, newError(ErrorConflictingValue, fieldTLSClientCert, nil)
	}
	for field, path := range map[string]string{
		fieldTLSRootCA: rootCA, fieldTLSClientCert: clientCert, fieldTLSClientKey: clientKey,
	} {
		if path != "" {
			if err := validateTLSPath(field, path, profile); err != nil {
				return TLSConfig{}, err
			}
		}
	}
	if mode == TLSVerifyFull && !dnsHost(host) && serverName == "" {
		return TLSConfig{}, newError(ErrorInvalidValue, fieldTLSServerName, nil)
	}
	if serverName != "" && !dnsHost(serverName) {
		return TLSConfig{}, newError(ErrorInvalidValue, fieldTLSServerName, nil)
	}
	return TLSConfig{mode: mode, rootCAPath: rootCA, clientCertPath: clientCert, clientKeyPath: clientKey, serverName: serverName}, nil
}

func validateTLSPath(field, path string, profile Profile) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return newError(ErrorInvalidValue, field, nil)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return newError(ErrorInvalidValue, field, err)
	}
	if !info.Mode().IsRegular() {
		return newError(ErrorInvalidValue, field, nil)
	}
	if profile == ProfileProduction && info.Mode()&os.ModeSymlink != 0 {
		return newError(ErrorUnsupported, field, nil)
	}
	if field == fieldTLSClientKey && runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return newError(ErrorUnsupported, field, nil)
	}
	return nil
}

func parseDuration(values map[string]string, field string, minimum, maximum time.Duration) (time.Duration, error) {
	value := values[field]
	if value == "" || !containsDurationUnit(value) {
		return 0, newError(ErrorInvalidValue, field, nil)
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < minimum || duration > maximum {
		return 0, newError(ErrorInvalidValue, field, err)
	}
	return duration, nil
}

func containsDurationUnit(value string) bool {
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || character == 'µ' {
			return true
		}
	}
	return false
}

func parseInteger(values map[string]string, field string, minimum, maximum int64) (int64, error) {
	value := values[field]
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, newError(ErrorInvalidValue, field, err)
	}
	return parsed, nil
}

func validateHost(host string) error {
	if host == "" || len(host) > maximumTextBytes || strings.ContainsAny(host, "\x00\r\n\t @") || strings.Contains(host, "://") {
		return newError(ErrorInvalidValue, fieldDBHost, nil)
	}
	if filepath.IsAbs(host) {
		if runtime.GOOS == "windows" {
			return newError(ErrorUnsupported, fieldDBHost, nil)
		}
		if filepath.Clean(host) != host {
			return newError(ErrorInvalidValue, fieldDBHost, nil)
		}
		return nil
	}
	if net.ParseIP(host) != nil || dnsHost(host) || strings.EqualFold(host, "localhost") {
		return nil
	}
	return newError(ErrorInvalidValue, fieldDBHost, nil)
}

func loopbackOrSocket(host string) bool {
	if runtime.GOOS != "windows" && filepath.IsAbs(host) {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func dnsHost(host string) bool {
	if host == "" || len(host) > 253 || net.ParseIP(host) != nil || filepath.IsAbs(host) {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
				(character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func safeIdentifier(value string) bool {
	if value == "" || len(value) > 63 {
		return false
	}
	for index, character := range value {
		if index == 0 && !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character == '_') {
			return false
		}
		if index > 0 && !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '_' || character == '-' || character == '.') {
			return false
		}
	}
	return true
}

func safeApplicationName(value string) bool {
	if value == "" || len([]byte(value)) > 64 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func safeServiceName(value string) bool { return safeApplicationName(value) }

func cloneSources(sources map[string]SourceCategory) map[string]SourceCategory {
	result := make(map[string]SourceCategory, len(sources))
	for field, source := range sources {
		result[field] = source
	}
	return result
}

func (profile Profile) valid() bool {
	switch profile {
	case ProfileLocal, ProfileCI, ProfileStaging, ProfileProduction:
		return true
	default:
		return false
	}
}

func (level LogLevel) valid() bool {
	switch level {
	case LogDebug, LogInfo, LogWarn, LogError:
		return true
	default:
		return false
	}
}

func (format LogFormat) valid() bool { return format == LogText || format == LogJSON }
func (mode TLSMode) valid() bool     { return mode == TLSDisabled || mode == TLSVerifyFull }

type environmentSecretProvider struct{ values map[SecretReference][]byte }

func (provider *environmentSecretProvider) Resolve(ctx context.Context, reference SecretReference) ([]byte, error) {
	if ctx == nil {
		return nil, newError(ErrorInvalidInput, "context", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, newError(ErrorCanceled, "context", err)
	}
	value, found := provider.values[reference]
	if !found || len(value) == 0 {
		return nil, newError(ErrorSecretUnavailable, "secret", nil)
	}
	return append([]byte(nil), value...), nil
}

// ValidateSecrets proves that every profile-required reference can be resolved
// without retaining secret bytes in the immutable configuration.
func (value LoadedConfiguration) ValidateSecrets(ctx context.Context) error {
	for _, reference := range value.configuration.RequiredSecrets() {
		secret, err := value.ResolveSecret(ctx, reference)
		if err != nil {
			return err
		}
		clear(secret)
	}
	return nil
}

var _ Loader = loader{}
var _ SecretProvider = (*environmentSecretProvider)(nil)
