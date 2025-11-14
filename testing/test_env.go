package testing

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/ingress"
	"github.com/restatedev/sdk-go/server"
	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	RESTATE_ADMIN_ENDPOINT_PORT   = "9070"
	RESTATE_INGRESS_ENDPOINT_PORT = "8080"
)

type TestEnvironment struct {
	t *testing.T

	srv           *httptest.Server
	adminPort     int
	ingressPort   int
	ingressClient *ingress.Client
}

// TestEnvironmentOption is a function that configures a TestEnvironment
type TestEnvironmentOption func(*testEnvironmentConfig)

type testEnvironmentConfig struct {
	restateEnv   map[string]string
	restateImage string
	followLogs   bool
}

// WithRestateEnv adds environment variables for the Restate service container
func WithRestateEnv(env map[string]string) TestEnvironmentOption {
	return func(c *testEnvironmentConfig) {
		for k, v := range env {
			c.restateEnv[k] = v
		}
	}
}

// WithRestateImage sets the Restate container image
func WithRestateImage(image string) TestEnvironmentOption {
	return func(c *testEnvironmentConfig) {
		c.restateImage = image
	}
}

// DisableRestateLogs disables restate log output
var DisableRestateLogs = func(c *testEnvironmentConfig) {
	c.followLogs = false
}

func defaultTestEnvironmentConfig() *testEnvironmentConfig {
	return &testEnvironmentConfig{
		restateEnv: map[string]string{
			"RUST_LOG": "warn",
		},
		restateImage: "docker.io/restatedev/restate:latest",
		followLogs:   true,
	}
}

// Start creates a test environment with Restate using testcontainers.
// It automatically:
// - Sets up an HTTP server for the provided Restate services
// - Starts a Restate container using testcontainers
// - Registers the services with Restate
// - Configures automatic cleanup at the end of the test
//
// The returned TestEnvironment provides access to the ingress client and ports
// for interacting with Restate during tests.
// For more options, use StartWithOptions.
//
// Example:
//
//	func TestMyService(t *testing.T) {
//		tEnv := Start(t, restate.Reflect(Greeter{}))
//		client := tEnv.Ingress()
//
//		out, err := ingress.Service[string, string](client, "Greeter", "Greet").Request(t.Context(), "Francesco")
//		require.NoError(t, err)
//		require.Equal(t, "You said hi to Francesco!", out)
//	}
func Start(t *testing.T, services ...restate.ServiceDefinition) *TestEnvironment {
	restateSrv := server.NewRestate()

	for _, service := range services {
		restateSrv.Bind(service)
	}

	return StartWithOptions(t, restateSrv)
}

// StartWithOptions creates a test environment with Restate using testcontainers, with custom configuration.
// It automatically:
// - Sets up an HTTP server for the provided Restate server
// - Starts a Restate container using testcontainers
// - Registers the services with Restate
// - Configures automatic cleanup at the end of the test (both HTTP server and container)
//
// Options allow you to customize the Restate container (e.g., image version, environment variables).
// The returned TestEnvironment provides access to the ingress client and ports for interacting with Restate during tests.
func StartWithOptions(t *testing.T, restateSrv *server.Restate, opts ...TestEnvironmentOption) *TestEnvironment {
	// Apply options
	config := defaultTestEnvironmentConfig()
	for _, opt := range opts {
		opt(config)
	}

	// These are overridden, the user cannot effectively set them
	config.restateEnv["RESTATE_META__REST_ADDRESS"] = "0.0.0.0:" + RESTATE_ADMIN_ENDPOINT_PORT
	config.restateEnv["RESTATE_WORKER__INGRESS__BIND_ADDRESS"] = "0.0.0.0:" + RESTATE_INGRESS_ENDPOINT_PORT

	// Start HTTP/2 server for serving the SDK
	restateHandler, err := restateSrv.Handler()
	require.NoError(t, err)
	srv := httptest.NewUnstartedServer(restateHandler)
	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	srv.Config.Protocols = &protocols
	srv.EnableHTTP2 = true
	srv.Start()
	t.Cleanup(func() {
		srv.Close()
	})
	// Figure out port
	sdkPort, err := strconv.Atoi(strings.Split(srv.URL, ":")[2])
	require.NoError(t, err)

	// Start restate container and configure cleanup
	restateC, err := testcontainers.Run(
		t.Context(), config.restateImage,
		testcontainers.WithEnv(config.restateEnv),
		testcontainers.WithExposedPorts(RESTATE_INGRESS_ENDPOINT_PORT+"/tcp", RESTATE_ADMIN_ENDPOINT_PORT+"/tcp"),
		testcontainers.WithWaitStrategyAndDeadline(
			time.Minute,
			wait.ForAll(
				wait.ForHTTP("/health").WithPort(RESTATE_ADMIN_ENDPOINT_PORT+"/tcp"),
				wait.ForHTTP("/restate/health").WithPort(RESTATE_INGRESS_ENDPOINT_PORT+"/tcp"),
			),
		),
		testcontainers.WithHostPortAccess(sdkPort),
	)
	testcontainers.CleanupContainer(t, restateC)
	require.NoError(t, err)

	if config.followLogs {
		reader, err := restateC.Logs(t.Context())
		require.NoError(t, err)
		go func() {
			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				t.Log(scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				t.Logf("Error when reading container logs: %e", err)
			}
		}()
	}

	adminPort, err := restateC.MappedPort(t.Context(), RESTATE_ADMIN_ENDPOINT_PORT)
	require.NoError(t, err)
	ingressPort, err := restateC.MappedPort(t.Context(), RESTATE_INGRESS_ENDPOINT_PORT)
	require.NoError(t, err)

	t.Log("Executing registration of port", sdkPort)
	res, err := http.Post(fmt.Sprintf("http://localhost:%d/deployments", adminPort.Int()), "application/json", bytes.NewBufferString(fmt.Sprintf("{\"uri\":\"http://%s:%d\"}", testcontainers.HostInternal, sdkPort)))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, res.StatusCode)

	ingressClient := ingress.NewClient(fmt.Sprintf("http://localhost:%d", ingressPort.Int()))

	return &TestEnvironment{
		t:             t,
		srv:           srv,
		adminPort:     adminPort.Int(),
		ingressPort:   ingressPort.Int(),
		ingressClient: ingressClient,
	}
}

func (tEnv *TestEnvironment) IngressPort() int {
	return tEnv.ingressPort
}

func (tEnv *TestEnvironment) AdminPort() int {
	return tEnv.adminPort
}

func (tEnv *TestEnvironment) Ingress() *ingress.Client {
	return tEnv.ingressClient
}
