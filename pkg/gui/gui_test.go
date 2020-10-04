package gui

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/stretchr/testify/assert"
)

// To run an integration test, e.g. for test 'commit', go:
// go test pkg/gui/gui_test.go -run /commit
//
// To record keypresses for an integration test, pass RECORD_EVENTS=true like so:
// RECORD_EVENTS=true go test pkg/gui/gui_test.go -run /commit
//
// To update a snapshot for an integration test, pass UPDATE_SNAPSHOT=true
// UPDATE_SNAPSHOT=true go test pkg/gui/gui_test.go -run /commit
//
// When RECORD_EVENTS is true, updates will be updated automatically
//
// integration tests are run in test/integration_test and the final test does
// not clean up that directory so you can cd into it to see for yourself what
// happened when a test failed.
//
// TODO: support passing an env var for playback speed, given it's currently pretty fast

type integrationTest struct {
	name    string
	fixture string
}

func tests() []integrationTest {
	return []integrationTest{
		{
			name:    "commit",
			fixture: "newFile",
		},
		{
			name:    "squash",
			fixture: "manyCommits",
		},
		{
			name:    "patchBuilding",
			fixture: "updatedFile",
		},
	}
}

func generateSnapshot(t *testing.T) string {
	osCommand := oscommands.NewDummyOSCommand()
	cmd := `sh -c "git status; cat ./*; git log --pretty=%B -p"`

	snapshot, err := osCommand.RunCommandWithOutput(cmd)
	assert.NoError(t, err)

	return snapshot
}

func findOrCreateDir(path string) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(path, 0777)
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}
}

func Test(t *testing.T) {
	tests := tests()

	gotoRootDirectory()

	rootDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			testPath := filepath.Join(rootDir, "test", "integration", test.name)
			findOrCreateDir(testPath)

			replayPath := filepath.Join(testPath, "recording.json")
			snapshotPath := filepath.Join(testPath, "snapshot.txt")

			err := os.Chdir(rootDir)
			assert.NoError(t, err)

			prepareIntegrationTestDir()

			err = createFixture(rootDir, test.fixture)
			assert.NoError(t, err)

			record := os.Getenv("RECORD_EVENTS") != ""
			runLazygit(t, replayPath, rootDir, record)

			updateSnapshot := record || os.Getenv("UPDATE_SNAPSHOT") != ""

			actual := generateSnapshot(t)

			if updateSnapshot {
				err := ioutil.WriteFile(snapshotPath, []byte(actual), 0600)
				assert.NoError(t, err)
			}

			expectedBytes, err := ioutil.ReadFile(snapshotPath)
			assert.NoError(t, err)
			expected := string(expectedBytes)

			assert.Equal(t, expected, actual, fmt.Sprintf("expected:\n%s\nactual:\n%s\n", expected, actual))
		})
	}
}

func createFixture(rootDir string, name string) error {
	osCommand := oscommands.NewDummyOSCommand()
	cmd := exec.Command("sh", filepath.Join(rootDir, "test", "fixtures", fmt.Sprintf("%s.sh", name)))

	if err := osCommand.RunExecutable(cmd); err != nil {
		return err
	}

	return nil
}

func gotoRootDirectory() {
	for {
		_, err := os.Stat(".git")

		if err == nil {
			return
		}

		if !os.IsNotExist(err) {
			panic(err)
		}

		if err = os.Chdir(".."); err != nil {
			panic(err)
		}
	}
}

func runLazygit(t *testing.T, replayPath string, rootDir string, record bool) {
	osCommand := oscommands.NewDummyOSCommand()

	var cmd *exec.Cmd
	if record {
		cmd = osCommand.ExecutableFromString(fmt.Sprintf("go run %s", filepath.Join(rootDir, "main.go")))
		cmd.Env = append(
			cmd.Env,
			fmt.Sprintf("RECORD_EVENTS_TO=%s", replayPath),
		)
	} else {
		cmd = osCommand.ExecutableFromString("lazygit")
		cmd.Env = append(
			cmd.Env,
			fmt.Sprintf("REPLAY_EVENTS_FROM=%s", replayPath),
		)
	}
	err := osCommand.RunExecutable(cmd)
	assert.NoError(t, err)
}

func prepareIntegrationTestDir() {
	path := filepath.Join("test", "integration_test")

	// remove contents of integration test directory
	dir, err := ioutil.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(path, 0777)
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}
	for _, d := range dir {
		os.RemoveAll(filepath.Join(path, d.Name()))
	}

	if err := os.Chdir(path); err != nil {
		panic(err)
	}
}
