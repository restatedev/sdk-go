package restate

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTerminal(t *testing.T) {
	require.False(t, IsTerminalError(fmt.Errorf("not terminal")))

	err := TerminalErrorf("failed terminally")
	require.True(t, IsTerminalError(err))

	//terminal with code
	err = TerminalError(fmt.Errorf("terminal with code"), 500)

	require.True(t, IsTerminalError(err))
	require.EqualValues(t, 500, ErrorCode(err))
}

func TestCode(t *testing.T) {

	err := WithErrorCode(fmt.Errorf("some error"), 16)

	code := ErrorCode(err)

	require.EqualValues(t, 16, code)

	require.EqualValues(t, http.StatusInternalServerError, ErrorCode(fmt.Errorf("unknown error")))
}

func TestCombine(t *testing.T) {
	err := WithErrorCode(TerminalError(fmt.Errorf("some error")), 100)

	require.True(t, IsTerminalError(err))
	require.EqualValues(t, 100, ErrorCode(err))
}
