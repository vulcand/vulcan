package vulcan

import (
	"flag"
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"os"
	"path/filepath"
)

type LogUtilsSuite struct {
	currentDir string
}

var _ = Suite(&LogUtilsSuite{})

func (s *LogUtilsSuite) SetUpTest(c *C) {
	tempDir, err := ioutil.TempDir(os.TempDir(), "vulcan_test")
	if err != nil {
		panic(err)
	}
	s.currentDir = tempDir
}

func (s *LogUtilsSuite) TearDownTest(c *C) {
	os.RemoveAll(s.currentDir)
}

func (s *LogUtilsSuite) createFiles(files []string, symlinks map[string]string, folders []string) {
	for _, fileName := range files {
		err := ioutil.WriteFile(filepath.Join(s.currentDir, fileName), []byte("hi"), 0644)
		if err != nil {
			panic(err)
		}
	}

	for in, out := range symlinks {
		err := os.Symlink(filepath.Join(s.currentDir, out), filepath.Join(s.currentDir, in))
		if err != nil {
			panic(err)
		}
	}

	for _, folderName := range folders {
		err := os.Mkdir(filepath.Join(s.currentDir, folderName), 0755)
		if err != nil {
			panic(err)
		}
	}
}

func (s *LogUtilsSuite) TestLogDir(c *C) {
	flag.Set("log_dir", "")
	c.Assert(LogDir(), Equals, "")
	flag.Set("log_dir", "/tmp/google")
	c.Assert(LogDir(), Equals, "/tmp/google")
}

func (s *LogUtilsSuite) TestProgramName(c *C) {
	c.Assert(programName(), Equals, "vulcan.test")
}

func (s *LogUtilsSuite) TestRemoveFiles(c *C) {
	s.createFiles(
		[]string{
			// old logs to be removed
			"vulcan.radar1.mg.log.INFO.20131004-221312.25058",
			"vulcan.radar1.mg.log.ERROR.20131005-005231.31443",
			"vulcan.radar1.mg.log.WARNING.20131005-005231.31443",

			// active logs referenced by symlinks
			"vulcan.radar1.mg.log.INFO.20131009-135124.365",
			"vulcan.radar1.mg.log.WARNING.20131005-011339.365",
			"vulcan.radar1.mg.log.ERROR.20131005-011339.365",

			// totally unrellated logs
			"mgcore-0.log",
			"mongo-radar.log.2012-12-07T05-26-56",
			"redis-test.log",
		},
		map[string]string{
			"vulcan.INFO":  "vulcan.radar1.mg.log.INFO.20131009-135124.365",
			"vulcan.WARN":  "vulcan.radar1.mg.log.WARNING.20131005-011339.365",
			"vulcan.ERROR": "vulcan.radar1.mg.log.ERROR.20131005-011339.365",
		},
		[]string{"vulcan.directory", "other.directory"},
	)
	removeFiles(s.currentDir, "vulcan")
	files, err := filepath.Glob(fmt.Sprintf("%s/*", s.currentDir))
	if err != nil {
		panic(err)
	}
	filesMap := make(map[string]bool, len(files))
	for _, path := range files {
		_, fileName := filepath.Split(path)
		filesMap[fileName] = true
	}
	expected := map[string]bool{
		"vulcan.radar1.mg.log.INFO.20131009-135124.365":    true,
		"vulcan.radar1.mg.log.WARNING.20131005-011339.365": true,
		"vulcan.radar1.mg.log.ERROR.20131005-011339.365":   true,
		"vulcan.INFO":                                      true,
		"vulcan.WARN":                                      true,
		"vulcan.ERROR":                                     true,
		"vulcan.directory":                                 true,
		"other.directory":                                  true,
	}
	for fileName, _ := range expected {
		if filesMap[fileName] != true {
			c.Errorf("Expected %s to be present", fileName)
		}
	}
}

func (s *LogUtilsSuite) TestCleanupLogs(c *C) {
	flag.Set("log_dir", s.currentDir)
	c.Assert(CleanupLogs(), IsNil)

	flag.Set("log_dir", "")
	c.Assert(CleanupLogs(), IsNil)

	flag.Set("log_dir", "/something/that/does/not/exist")
	c.Assert(CleanupLogs(), IsNil)
}
