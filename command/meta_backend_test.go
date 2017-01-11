package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/cli"
)

// Test empty directory with no config/state creates a local state.
func TestMetaBackend_emptyDir(t *testing.T) {
	// Create a temporary working directory that is empty
	td := tempDir(t)
	os.MkdirAll(td, 0755)
	defer os.RemoveAll(td)
	defer testChdir(t, td)()

	// Get the backend
	m := testMetaBackend(t, nil)
	b, err := m.Backend(nil)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Write some state
	s, err := b.State()
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	s.WriteState(testState())
	if err := s.PersistState(); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Verify it exists where we expect it to
	if _, err := os.Stat(DefaultStateFilename); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify no backup since it was empty to start
	if _, err := os.Stat(DefaultStateFilename + DefaultBackupExtension); err == nil {
		t.Fatalf("err: %s", err)
	}

	// Verify no backend state was made
	if _, err := os.Stat(filepath.Join(m.DataDir(), DefaultStateFilename)); err == nil {
		t.Fatalf("err: %s", err)
	}
}

// Test a directory with a legacy state and no config continues to
// use the legacy state.
func TestMetaBackend_emptyWithDefaultState(t *testing.T) {
	// Create a temporary working directory that is empty
	td := tempDir(t)
	os.MkdirAll(td, 0755)
	defer os.RemoveAll(td)
	defer testChdir(t, td)()

	// Write the legacy state
	statePath := DefaultStateFilename
	{
		f, err := os.Create(statePath)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		err = terraform.WriteState(testState(), f)
		f.Close()
		if err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Get the backend
	m := testMetaBackend(t, nil)
	b, err := m.Backend(nil)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Check the state
	s, err := b.State()
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	if actual := s.State().String(); actual != testState().String() {
		t.Fatalf("bad: %s", actual)
	}

	// Verify it exists where we expect it to
	if _, err := os.Stat(DefaultStateFilename); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(filepath.Join(m.DataDir(), DefaultStateFilename)); err == nil {
		t.Fatalf("err: %s", err)
	}

	// Write some state
	next := testState()
	next.Modules[0].Outputs["foo"] = &terraform.OutputState{Value: "bar"}
	s.WriteState(testState())
	if err := s.PersistState(); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Verify a backup was made since we're modifying a pre-existing state
	if _, err := os.Stat(DefaultStateFilename + DefaultBackupExtension); err != nil {
		t.Fatalf("err: %s", err)
	}
}

// Test an empty directory with an explicit state path (outside the dir)
func TestMetaBackend_emptyWithExplicitState(t *testing.T) {
	// Create a temporary working directory that is empty
	td := tempDir(t)
	os.MkdirAll(td, 0755)
	defer os.RemoveAll(td)
	defer testChdir(t, td)()

	// Create another directory to store our state
	stateDir := tempDir(t)
	os.MkdirAll(stateDir, 0755)
	defer os.RemoveAll(stateDir)

	// Write the legacy state
	statePath := filepath.Join(stateDir, "foo")
	{
		f, err := os.Create(statePath)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		err = terraform.WriteState(testState(), f)
		f.Close()
		if err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Setup the meta
	m := testMetaBackend(t, nil)
	m.statePath = statePath

	// Get the backend
	b, err := m.Backend(nil)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Check the state
	s, err := b.State()
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	if actual := s.State().String(); actual != testState().String() {
		t.Fatalf("bad: %s", actual)
	}

	// Verify neither defaults exist
	if _, err := os.Stat(DefaultStateFilename); err == nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(filepath.Join(m.DataDir(), DefaultStateFilename)); err == nil {
		t.Fatalf("err: %s", err)
	}

	// Write some state
	next := testState()
	next.Modules[0].Outputs["foo"] = &terraform.OutputState{Value: "bar"}
	s.WriteState(testState())
	if err := s.PersistState(); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Verify a backup was made since we're modifying a pre-existing state
	if _, err := os.Stat(statePath + DefaultBackupExtension); err != nil {
		t.Fatalf("err: %s", err)
	}
}

// Empty directory with legacy remote state
func TestMetaBackend_emptyLegacyRemote(t *testing.T) {
	// Create a temporary working directory that is empty
	td := tempDir(t)
	os.MkdirAll(td, 0755)
	defer os.RemoveAll(td)
	defer testChdir(t, td)()

	// Create some legacy remote state
	legacyState := testState()
	_, srv := testRemoteState(t, legacyState, 200)
	defer srv.Close()
	statePath := testStateFileRemote(t, legacyState)

	// Setup the meta
	m := testMetaBackend(t, nil)

	// Get the backend
	b, err := m.Backend(nil)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Check the state
	s, err := b.State()
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	if err := s.RefreshState(); err != nil {
		t.Fatalf("bad: %s", err)
	}
	state := s.State()
	if actual := state.String(); actual != legacyState.String() {
		t.Fatalf("bad: %s", actual)
	}

	// Verify we didn't setup the backend state
	if !state.Backend.Empty() {
		t.Fatal("shouldn't configure backend")
	}

	// Verify the default paths don't exist
	if _, err := os.Stat(DefaultStateFilename); err == nil {
		t.Fatalf("err: %s", err)
	}

	// Verify a backup doesn't exist
	if _, err := os.Stat(DefaultStateFilename + DefaultBackupExtension); err == nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(statePath + DefaultBackupExtension); err == nil {
		t.Fatalf("err: %s", err)
	}
}

func testMetaBackend(t *testing.T, args []string) *Meta {
	var m Meta
	m.Ui = new(cli.MockUi)
	m.process(args, true)
	f := m.flagSet("test")
	if err := f.Parse(args); err != nil {
		t.Fatalf("bad: %s", err)
	}

	return &m
}
