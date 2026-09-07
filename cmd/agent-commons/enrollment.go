package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"agentcommons/internal/core"
)

type enrollmentConnection struct {
	Path   string
	Config connectionConfig
}

func enrollAgent(ctx context.Context, options onboardingOptions, out io.Writer) error {
	connection, err := prepareEnrollment(options)
	if err != nil {
		return err
	}
	exists, err := connection.checkExisting()
	if err != nil {
		return err
	}
	credential, err := connection.enrollIdentity(ctx, options.Team)
	if err != nil {
		return err
	}
	if err = ensureCredential(connection.Config.TokenFile, credential); err != nil {
		return err
	}
	if !exists {
		if err = connection.writeConfig(); err != nil {
			return err
		}
	}
	return json.NewEncoder(out).Encode(map[string]string{
		"identity": connection.Config.Identity, "config": connection.Path, "target": connection.Config.Target,
	})
}

func prepareEnrollment(options onboardingOptions) (enrollmentConnection, error) {
	if options.Name == "" || options.Role == "" || !filepath.IsAbs(options.Target) {
		return enrollmentConnection{}, errors.New("enroll requires --name --role and absolute --target")
	}
	target, err := filepath.EvalSymlinks(options.Target)
	if err != nil {
		return enrollmentConnection{}, err
	}
	if options.Identity == "" {
		options.Identity = "agent-" + identityDigest(target+"\x00"+options.Name+"\x00"+options.Role)
	}
	stem := identityDigest(options.Identity)
	if options.Config == "" {
		options.Config = filepath.Join(options.State, "connection-"+stem+".json")
	}
	connection := enrollmentConnection{Path: options.Config, Config: connectionConfig{
		Version: 1, Identity: options.Identity, Target: target, Name: options.Name, Role: options.Role,
		Socket: filepath.Join(options.State, "service.sock"), State: options.State,
		TokenFile: filepath.Join(filepath.Dir(options.Config), "credential-"+stem+".token"),
	}}
	return connection, validateConfigLocation(connection.Path)
}

func identityDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:16])
}

func validateConfigLocation(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("absolute --config required")
	}
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("config parent must be a real private directory")
	}
	return nil
}

func (connection enrollmentConnection) checkExisting() (bool, error) {
	if _, err := os.Lstat(connection.Path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	var previous connectionConfig
	if err := privateRead(connection.Path, &previous); err != nil {
		return false, err
	}
	if previous != connection.Config {
		return false, errors.New("existing config differs; refusing overwrite")
	}
	return true, nil
}

func (connection enrollmentConnection) enrollIdentity(ctx context.Context, team string) (string, error) {
	config := connection.Config
	operator, err := readToken(filepath.Join(config.State, "operator.token"))
	if err != nil {
		return "", err
	}
	identity := core.Session{ID: config.Identity, Name: config.Name, Role: config.Role, Team: team,
		Target: config.Target, Runtime: "manual", Mode: "manual", Policy: "coordination"}
	result, err := rpcCall[struct {
		Token string `json:"token"`
	}](ctx,
		rpcClient{socket: config.Socket, token: operator}, "sessions.enroll", identity)
	if err != nil {
		return "", err
	}
	if result.Token == "" {
		return "", errors.New("missing enrollment credential")
	}
	return result.Token, nil
}

func ensureCredential(path, expected string) error {
	existing, err := readToken(path)
	if errors.Is(err, os.ErrNotExist) {
		return createPrivate(path, []byte(expected+"\n"))
	}
	if err != nil {
		return err
	}
	if existing != expected {
		return errors.New("credential mismatch; refusing overwrite")
	}
	return nil
}

func (connection enrollmentConnection) writeConfig() error {
	data, err := json.MarshalIndent(connection.Config, "", "  ")
	if err != nil {
		return err
	}
	return createPrivate(connection.Path, data)
}
