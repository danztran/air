package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/imdario/mergo"
	"github.com/spf13/viper"
)

const (
	dftTOML = ".air.toml"
	dftYAML = ".air.yaml"
	dftConf = ".air.conf"
	airWd   = "air_wd"
)

type config struct {
	Root   string   `toml:"root" mapstructure:"root"`
	TmpDir string   `toml:"tmp_dir" mapstructure:"tmp_dir"`
	Build  cfgBuild `toml:"build" mapstructure:"build"`
	Color  cfgColor `toml:"color" mapstructure:"color"`
	Log    cfgLog   `toml:"log" mapstructure:"log"`
	Misc   cfgMisc  `toml:"misc" mapstructure:"misc"`
}

type cfgBuild struct {
	Cmd           string        `toml:"cmd" mapstructure:"cmd"`
	Bin           string        `toml:"bin" mapstructure:"bin"`
	FullBin       string        `toml:"full_bin" mapstructure:"full_bin"`
	Log           string        `toml:"log" mapstructure:"log"`
	IncludeExt    []string      `toml:"include_ext" mapstructure:"include_ext"`
	ExcludeDir    []string      `toml:"exclude_dir" mapstructure:"exclude_dir"`
	IncludeDir    []string      `toml:"include_dir" mapstructure:"include_dir"`
	ExcludeFile   []string      `toml:"exclude_file" mapstructure:"exclude_file"`
	Delay         int           `toml:"delay" mapstructure:"delay"`
	StopOnError   bool          `toml:"stop_on_error" mapstructure:"stop_on_error"`
	SendInterrupt bool          `toml:"send_interrupt" mapstructure:"send_interrupt"`
	KillDelay     time.Duration `toml:"kill_delay" mapstructure:"kill_delay"`
}

type cfgLog struct {
	AddTime bool `toml:"time" mapstructure:"time"`
}

type cfgColor struct {
	Main    string `toml:"main" mapstructure:"main"`
	Watcher string `toml:"watcher" mapstructure:"watcher"`
	Build   string `toml:"build" mapstructure:"build"`
	Runner  string `toml:"runner" mapstructure:"runner"`
	App     string `toml:"app" mapstructure:"app"`
}

type cfgMisc struct {
	CleanOnExit bool `toml:"clean_on_exit" mapstructure:"clean_on_exit"`
}

func initConfig(path string) (cfg *config, err error) {
	if path == "" {
		cfg, err = defaultPathConfig()
		if err != nil {
			return nil, err
		}
	} else {
		cfg, err = readConfigOrDefault(path)
		if err != nil {
			return nil, err
		}
	}
	err = mergo.Merge(cfg, defaultConfig())
	if err != nil {
		return nil, err
	}
	err = cfg.preprocess()
	return cfg, err
}

func defaultPathConfig() (*config, error) {
	// when path is blank, first find `.air.toml`, `.air.conf` in `air_wd` and current working directory, if not found, use defaults
	for _, name := range []string{dftYAML, dftTOML, dftConf} {
		cfg, err := readConfByName(name)
		if err == nil {
			if name == dftConf {
				fmt.Println("`.air.conf` will be deprecated soon, recommend using `.air.toml`.")
			}
			return cfg, nil
		}
	}

	dftCfg := defaultConfig()
	return &dftCfg, nil
}

func readConfByName(name string) (*config, error) {
	var path string
	if wd := os.Getenv(airWd); wd != "" {
		path = filepath.Join(wd, name)
	} else {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(wd, name)
	}
	cfg, err := readConfig(path)
	return cfg, err
}

func defaultConfig() config {
	build := cfgBuild{
		Cmd:         "go build -o ./tmp/main .",
		Bin:         "./tmp/main",
		Log:         "build-errors.log",
		IncludeExt:  []string{"go", "tpl", "tmpl", "html"},
		ExcludeDir:  []string{"assets", "tmp", "vendor"},
		Delay:       1000,
		StopOnError: true,
	}
	if runtime.GOOS == PlatformWindows {
		build.Bin = `tmp\main.exe`
		build.Cmd = "go build -o ./tmp/main.exe ."
	}
	log := cfgLog{
		AddTime: false,
	}
	color := cfgColor{
		Main:    "magenta",
		Watcher: "cyan",
		Build:   "yellow",
		Runner:  "green",
	}
	misc := cfgMisc{
		CleanOnExit: false,
	}
	return config{
		Root:   ".",
		TmpDir: "tmp",
		Build:  build,
		Color:  color,
		Log:    log,
		Misc:   misc,
	}
}

func readConfig(path string) (*config, error) {
	cfg := new(config)
	v := viper.New()

	v.SetConfigFile(path)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()

	err := v.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("error read config / %w", err)
	}

	err = v.Unmarshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("error parse config / %w", err)
	}

	return cfg, nil
}

func readConfigOrDefault(path string) (*config, error) {
	dftCfg := defaultConfig()
	cfg, err := readConfig(path)
	if err != nil {
		return &dftCfg, err
	}

	return cfg, nil
}

func (c *config) preprocess() error {
	var err error
	cwd := os.Getenv(airWd)
	if cwd != "" {
		if err = os.Chdir(cwd); err != nil {
			return err
		}
		c.Root = cwd
	}
	c.Root, err = expandPath(c.Root)
	if c.TmpDir == "" {
		c.TmpDir = "tmp"
	}
	if err != nil {
		return err
	}
	ed := c.Build.ExcludeDir
	for i := range ed {
		ed[i] = cleanPath(ed[i])
	}

	adaptToVariousPlatforms(c)

	c.Build.ExcludeDir = ed
	if len(c.Build.FullBin) > 0 {
		c.Build.Bin = c.Build.FullBin
		return err
	}
	// Fix windows CMD processor
	// CMD will not recognize relative path like ./tmp/server
	c.Build.Bin, err = filepath.Abs(c.Build.Bin)
	return err
}

func (c *config) colorInfo() map[string]string {
	return map[string]string{
		"main":    c.Color.Main,
		"build":   c.Color.Build,
		"runner":  c.Color.Runner,
		"watcher": c.Color.Watcher,
	}
}

func (c *config) buildLogPath() string {
	return filepath.Join(c.tmpPath(), c.Build.Log)
}

func (c *config) buildDelay() time.Duration {
	return time.Duration(c.Build.Delay) * time.Millisecond
}

func (c *config) binPath() string {
	return filepath.Join(c.Root, c.Build.Bin)
}

func (c *config) tmpPath() string {
	return filepath.Join(c.Root, c.TmpDir)
}

func (c *config) rel(path string) string {
	s, err := filepath.Rel(c.Root, path)
	if err != nil {
		return ""
	}
	return s
}
