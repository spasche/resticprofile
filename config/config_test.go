package config

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testTemplate struct {
	format string
	config string
}

func TestIsSet(t *testing.T) {
	testData := []testTemplate{
		{FormatTOML, `
[first]
[first.second]
key="value"
`},
		{FormatJSON, `
{
  "first": {
    "second": {
		"key": "value"
	}
  }
}`},
		{FormatYAML, `---
first:
  second:
    key: value
`},
	}

	for _, testItem := range testData {
		format := testItem.format
		testConfig := testItem.config
		t.Run(format, func(t *testing.T) {
			c, err := Load(bytes.NewBufferString(testConfig), format)
			require.NoError(t, err)

			assert.True(t, c.IsSet("first"))
			assert.True(t, c.IsSet("first.second"))
			assert.True(t, c.IsSet("first.second.key"))
		})
	}
}

func TestGetGlobal(t *testing.T) {
	testData := []testTemplate{
		{FormatTOML, `
[global]
priority = "low"
default-command = "version"
# initialize a repository if none exist at location
initialize = false
`},
		{FormatJSON, `
{
  "global": {
    "default-command": "version",
    "initialize": false,
    "priority": "low"
  }
}`},
		{FormatYAML, `---
global:
    default-command: version
    initialize: false
    priority: low
`},
		{FormatHCL, `
"global" = {
    default-command = "version"
    initialize = false
    priority = "low"
}
`},
		{FormatHCL, `
"global" = {
    default-command = "version"
    initialize = true
}

"global" = {
    initialize = false
    priority = "low"
}
`},
		{FormatTOML, `
version = 2
[global]
priority = "low"
default-command = "version"
# initialize a repository if none exist at location
initialize = false
`},
		{FormatJSON, `
{
  "version": 2,
  "global": {
    "default-command": "version",
    "initialize": false,
    "priority": "low"
  }
}`},
		{FormatYAML, `---
version: 2
global:
    default-command: version
    initialize: false
    priority: low
`},
		{FormatHCL, `
version = 2
"global" = {
    default-command = "version"
    initialize = false
    priority = "low"
}
`},
		{FormatHCL, `
version = 2
"global" = {
    default-command = "version"
    initialize = true
}

"global" = {
    initialize = false
    priority = "low"
}
`},
	}

	for _, testItem := range testData {
		format := testItem.format
		testConfig := testItem.config
		t.Run(format, func(t *testing.T) {
			c, err := Load(bytes.NewBufferString(testConfig), format)
			require.NoError(t, err)

			global, err := c.GetGlobalSection()
			require.NoError(t, err)
			assert.Equal(t, "version", global.DefaultCommand)
			assert.Equal(t, false, global.Initialize)
			assert.Equal(t, "low", global.Priority)
			assert.Equal(t, false, global.IONice)
		})
	}
}

func TestStringWithCommaNotConvertedToSlice(t *testing.T) {
	testData := []testTemplate{
		{"toml", `
[profile]
run-before = "first, second, third"
run-after = ["first", "second", "third"]
`},
		{"json", `
{
  "profile": {
    "run-before": "first, second, third",
    "run-after": ["first", "second", "third"]
  }
}`},
		{"yaml", `---
profile:
    run-before: first, second, third
    run-after: ["first", "second", "third"]
`},
		{"hcl", `
"profile" = {
    run-before = "first, second, third"
    run-after = ["first", "second", "third"]
}
`},
	}

	for _, testItem := range testData {
		format := testItem.format
		testConfig := testItem.config
		t.Run(format, func(t *testing.T) {
			c, err := Load(bytes.NewBufferString(testConfig), format)
			require.NoError(t, err)

			profile, err := c.GetProfile("profile")
			require.NoError(t, err)

			assert.NotNil(t, profile)
			assert.Len(t, profile.RunBefore, 1)
			assert.Len(t, profile.RunAfter, 3)
		})
	}
}

func TestGetIncludes(t *testing.T) {
	config, err := Load(bytes.NewBufferString(`includes=["i1", "i2"]`), "toml")
	require.NoError(t, err)
	assert.Equal(t, config.getIncludes(), []string{"i1", "i2"})

	config, err = Load(bytes.NewBufferString(`includes="inc"`), "toml")
	require.NoError(t, err)
	assert.Equal(t, config.getIncludes(), []string{"inc"})

	config, err = Load(bytes.NewBufferString(`
[includes]
x=0
	`), "toml")
	require.NoError(t, err)
	assert.Nil(t, config.getIncludes())

	config, err = Load(bytes.NewBufferString(`x=0`), "toml")
	require.NoError(t, err)
	assert.Nil(t, config.getIncludes())
}

func TestIncludes(t *testing.T) {
	files := []string{}
	cleanFiles := func() {
		for _, file := range files {
			os.Remove(file)
		}
		files = files[:0]
	}
	defer cleanFiles()

	createFile := func(t *testing.T, suffix, content string) string {
		t.Helper()
		name := ""
		file, err := os.CreateTemp("", "*-"+suffix)
		if err == nil {
			defer file.Close()
			_, err = file.WriteString(content)
			name = file.Name()
			files = append(files, name)
		}
		require.NoError(t, err)
		return name
	}

	mustLoadConfig := func(t *testing.T, configFile string) *Config {
		t.Helper()
		config, err := LoadFile(configFile, "")
		require.NoError(t, err)
		return config
	}

	testID := fmt.Sprintf("%d", time.Now().Unix())

	t.Run("multiple-includes", func(t *testing.T) {
		defer cleanFiles()
		content := fmt.Sprintf(`includes=['*%[1]s.inc.toml','*%[1]s.inc.yaml','*%[1]s.inc.json']`, testID)

		configFile := createFile(t, "profiles.conf", content)
		createFile(t, "d-"+testID+".inc.toml", `[one]`)
		createFile(t, "o-"+testID+".inc.yaml", `two: {}`)
		createFile(t, "j-"+testID+".inc.json", `{"three":{}}`)

		config := mustLoadConfig(t, configFile)
		assert.True(t, config.IsSet("includes"))
		assert.True(t, config.HasProfile("one"))
		assert.True(t, config.HasProfile("two"))
		assert.True(t, config.HasProfile("three"))
	})

	t.Run("overrides", func(t *testing.T) {
		defer cleanFiles()

		configFile := createFile(t, "profiles.conf", `
includes = "*`+testID+`.inc.toml"
[default]
repository = "default-repo"`)

		createFile(t, "override-"+testID+".inc.toml", `
[default]
repository = "overridden-repo"`)

		config := mustLoadConfig(t, configFile)
		assert.True(t, config.HasProfile("default"))

		profile, err := config.GetProfile("default")
		assert.NoError(t, err)
		assert.Equal(t, NewConfidentialValue("overridden-repo"), profile.Repository)
	})

	t.Run("hcl-includes-only-hcl", func(t *testing.T) {
		defer cleanFiles()

		configFile := createFile(t, "profiles.hcl", `includes = "*`+testID+`.inc.*"`)
		createFile(t, "pass-"+testID+".inc.hcl", `one { }`)

		config := mustLoadConfig(t, configFile)
		assert.True(t, config.HasProfile("one"))

		createFile(t, "fail-"+testID+".inc.toml", `[two]`)
		_, err := LoadFile(configFile, "")
		assert.Error(t, err)
		assert.Regexp(t, ".+ is in hcl format, includes must use the same format", err.Error())
	})

	t.Run("non-hcl-include-no-hcl", func(t *testing.T) {
		defer cleanFiles()

		configFile := createFile(t, "profiles.toml", `includes = "*`+testID+`.inc.*"`)
		createFile(t, "pass-"+testID+".inc.toml", `[one]`)

		config := mustLoadConfig(t, configFile)
		assert.True(t, config.HasProfile("one"))

		createFile(t, "fail-"+testID+".inc.hcl", `one { }`)
		_, err := LoadFile(configFile, "")
		assert.Error(t, err)
		assert.Regexp(t, "hcl format .+ cannot be used in includes from toml", err.Error())
	})

	t.Run("cannot-load-different-versions", func(t *testing.T) {
		defer cleanFiles()
		content := fmt.Sprintf(`includes=['*%s.inc.json']`, testID)

		configFile := createFile(t, "profiles.conf", content)
		createFile(t, "a-"+testID+".inc.json", `{"version": 2, "profiles": {"one":{}}}`)
		createFile(t, "b-"+testID+".inc.json", `{"two":{}}`)

		_, err := LoadFile(configFile, "")
		assert.Error(t, err)
	})

	t.Run("cannot-load-different-versions", func(t *testing.T) {
		defer cleanFiles()
		content := fmt.Sprintf(`{"version": 2, "includes"=[\"*%s.inc.json\"}]`, testID)

		configFile := createFile(t, "profiles.json", content)
		createFile(t, "c-"+testID+".inc.json", `{"two":{}}`)
		createFile(t, "d-"+testID+".inc.json", `{"version": 2, "profiles": {"one":{}}}`)

		_, err := LoadFile(configFile, "")
		assert.Error(t, err)
	})
}
