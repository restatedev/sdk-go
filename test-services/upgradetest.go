package main

import (
	"fmt"
	"os"
	"strings"

	restate "github.com/restatedev/sdk-go"
)

func init() {
	version := func() string {
		return strings.TrimSpace(os.Getenv("E2E_UPGRADETEST_VERSION"))
	}
	REGISTRY.AddDefinition(
		restate.NewService("UpgradeTest").
			Handler("executeSimple", restate.NewServiceHandler(
				func(ctx restate.Context, _ restate.Void) (string, error) {
					return version(), nil
				})).
			Handler("executeComplex", restate.NewServiceHandler(
				func(ctx restate.Context, _ restate.Void) (string, error) {
					if version() != "v1" {
						return "", fmt.Errorf("executeComplex should not be invoked with version different from 1!")
					}
					awakeable := restate.AwakeableAs[string](ctx)
					ctx.Object("AwakeableHolder", "upgrade", "hold").Send(awakeable.Id(), 0)
					if _, err := awakeable.Result(); err != nil {
						return "", err
					}
					ctx.Object("ListObject", "upgrade-test", "append").Send(version(), 0)
					return version(), nil
				})))
}
